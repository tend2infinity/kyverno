package policycache

import (
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"k8s.io/client-go/tools/cache"
)

// Controller is responsible for synchronizing Policy Cache,
// it embeds a policy informer to handle policy events.
// The cache is synced when a policy is add/update/delete.
// This cache is only used in the admission webhook to fast retrieve
// policies based on types (Mutate/ValidateEnforce/Generate).
type Controller struct {
	pSynched   cache.InformerSynced
	nspSynched cache.InformerSynced
	Cache      Interface
	log        logr.Logger
}

// NewPolicyCacheController create a new PolicyController
func NewPolicyCacheController(
	pInformer kyvernoinformer.ClusterPolicyInformer,
	nspInformer kyvernoinformer.PolicyInformer,
	log logr.Logger) *Controller {

	pc := Controller{
		Cache: newPolicyCache(log, pInformer.Lister(), nspInformer.Lister()),
		log:   log,
	}

	// ClusterPolicy Informer
	pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})

	// Policy Informer
	nspInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addNsPolicy,
		UpdateFunc: pc.updateNsPolicy,
		DeleteFunc: pc.deleteNsPolicy,
	})

	pc.pSynched = pInformer.Informer().HasSynced
	pc.nspSynched = nspInformer.Informer().HasSynced

	return &pc
}

func (c *Controller) addPolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	c.Cache.add(p)
}

func (c *Controller) updatePolicy(old, cur interface{}) {
	pOld := old.(*kyverno.ClusterPolicy)
	pNew := cur.(*kyverno.ClusterPolicy)
	if reflect.DeepEqual(pOld.Spec, pNew.Spec) {
		return
	}
	c.Cache.remove(pOld)
	c.Cache.add(pNew)
}

func (c *Controller) deletePolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	c.Cache.remove(p)
}

// addNsPolicy - Add Policy to cache
func (c *Controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyverno.Policy)
	c.Cache.add(p)
}

// updateNsPolicy - Update Policy of cache
func (c *Controller) updateNsPolicy(old, cur interface{}) {
	npOld := old.(*kyverno.Policy)
	npNew := cur.(*kyverno.Policy)
	if reflect.DeepEqual(npOld.Spec, npNew.Spec) {
		return
	}
	c.Cache.remove(npOld)
	c.Cache.add(npNew)
}

// deleteNsPolicy - Delete Policy from cache
func (c *Controller) deleteNsPolicy(obj interface{}) {
	p := obj.(*kyverno.Policy)
	c.Cache.remove(p)
}

// Run waits until policy informer to be synced
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	logger := c.log
	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForCacheSync(stopCh, c.pSynched) {
		logger.Info("failed to sync informer cache")
		return
	}

	<-stopCh
}

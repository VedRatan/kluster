package controller

import (
	"context"
	"log"
	"time"

	"github.com/VedRatan/kluster/pkg/apis/vedratan.dev/v1alpha1"
	klientset "github.com/VedRatan/kluster/pkg/client/clientset/versioned"
	kinformer "github.com/VedRatan/kluster/pkg/client/informers/externalversions/vedratan.dev/v1alpha1"
	klister "github.com/VedRatan/kluster/pkg/client/listers/vedratan.dev/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"github.com/VedRatan/kluster/pkg/digitalocean"
	"github.com/kanisterio/kanister/pkg/poll"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct{

	client kubernetes.Interface

	//clientset for custom resource controller
	klient klientset.Interface

	//kluster has synced
	klusterSynced cache.InformerSynced

	//lister
	klister klister.KlusterLister

	//queue
	wq workqueue.RateLimitingInterface

	recorder record.EventRecorder
}

func NewController(client kubernetes.Interface, klient klientset.Interface, klusterInformer kinformer.KlusterInformer) *Controller{
	c := &Controller{
		client: client,
		klient: klient,
		klusterSynced: klusterInformer.Informer().HasSynced,
		klister: klusterInformer.Lister(),
		wq: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "kluster"),
	}

	klusterInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleAdd,
			DeleteFunc: c.handleDel,
		},
	)

	return c
}

func (c *Controller) Run(ch chan struct{}) error{
	if ok := cache.WaitForCacheSync(ch, c.klusterSynced); !ok {
		log.Println("cache is not synced")
	}
	go wait.Until(c.worker, time.Second, ch)
	<-ch
	return nil
}

func (c *Controller) worker(){
	for c.processNextItem(){

	}
}

func (c *Controller) processNextItem() bool {
	item, shutDown := c.wq.Get()
	if shutDown {
		// logs as well
		return false
	}

	defer c.wq.Forget(item)
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		log.Printf("error %s calling Namespace key func on cache for item", err.Error())
		return false
	}

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.Printf("splitting key into namespace and name, error %s\n", err.Error())
		return false
	}


	kluster, err := c.klister.Klusters(ns).Get(name)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return deleteDOCluster()
		}

		log.Printf("error %s, Getting the kluster resource from lister", err.Error())
		return false
	}

	log.Printf("kluster specs are: %+v\n", kluster.Spec)
	
	clusterID, err := digitalocean.Create(c.client, kluster.Spec)

	log.Printf("cluster ID on digitalocean: %s\n", clusterID)
	if err != nil {
		log.Printf("error %s, in creating kluster %s", err.Error(), kluster.Name)
	}

	err = c.updateStatus(clusterID, "creating", kluster)
	if err != nil {
		log.Printf("error %s, in updating the status of the kluster %s", err.Error(), kluster.Name)
	}

	// query digial ocean api to make sure that the cluster is up and running

	err = c.waitForCluster(kluster.Spec, clusterID)
	if err != nil {
		log.Printf("error %s, waiting for cluster to be running", err.Error())
	}

	err = c.updateStatus(clusterID, "running", kluster)
	if err != nil {
		log.Printf("error %s updaring cluster status after waiting for cluster", err.Error())
	}

 	return true
}

func (c *Controller) waitForCluster(spec v1alpha1.KlusterSpec, clusterID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	return poll.Wait(ctx, func(ctx context.Context) (bool, error) {
		state, err := digitalocean.ClusterState(c.client, spec, clusterID)
		if err != nil {
			return false, err
		}
		if state == "running" {
			return true, nil
		}

		return false, nil
	})
}



func (c *Controller) updateStatus(id, progress string, kluster *v1alpha1.Kluster) error {

	// get the latest version of the cluster
	 k, err := c.klient.VedratanV1alpha1().Klusters(kluster.Namespace).Get(context.Background(), kluster.Name, metav1.GetOptions{})
	 if err != nil {
		return err
	 }

	k.Status.KlusterID = id
	k.Status.Progress = progress
	_, err = c.klient.VedratanV1alpha1().Klusters(kluster.Namespace).UpdateStatus(context.Background(), k, metav1.UpdateOptions{} )
	return err
}


func deleteDOCluster() bool {
	// this actualy deletes the cluster from the DO
	log.Println("Cluster was deleted succcessfully")
	return true
}

func (c *Controller) handleAdd(obj interface{}){
	log.Println("add was called")
	c.wq.Add(obj)
}

func (c *Controller) handleDel(obj interface{}){
	log.Println("delete was called")
	c.wq.Add(obj)
}


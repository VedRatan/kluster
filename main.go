package main

import (
	"flag"
	"log"
	"path/filepath"
	"time"

	klient "github.com/VedRatan/kluster/pkg/client/clientset/versioned"
	klusterInformerFactory "github.com/VedRatan/kluster/pkg/client/informers/externalversions"
	"github.com/VedRatan/kluster/pkg/controller"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)
 func main() {

	var kubeconfig *string

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Printf("Building config from flags failed, %s, trying to build inclusterconfig", err.Error())
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Printf("error %s building inclusterconfig", err.Error())
		}
	}

	klientset, err := klient.NewForConfig(config)
	if err != nil {
		log.Printf("getting klient set %s\n", err.Error())
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("getting std client %s\n", err.Error())
	}

	infofactory := klusterInformerFactory.NewSharedInformerFactory(klientset, 20*time.Minute)
	ch := make(chan struct{})
	
	c := controller.NewController(client, klientset, infofactory.Vedratan().V1alpha1().Klusters())

	infofactory.Start(ch)
	if err := c.Run(ch); err != nil {
		log.Printf("error in running controller: %s\n", err.Error())
	}
	// fmt.Println(klientset)

	//checking the number of kluster resources present in the cluster

	// klusters, err := klientset.VedratanV1alpha1().Klusters("").List(context.Background(), v1.ListOptions{})
	// if err !=nil {
	// 	log.Printf("listing klusters %s\n", err.Error())
	// }
	// fmt.Printf("length of klusters is: %d\n", len(klusters.Items))
 }
package digitalocean

// for reference
// apiVersion: v1
// data:
//   token: MjgzNDc4Mjg0NzJqa2hkZmprOTgyMw==
// kind: Secret
// metadata:
//   creationTimestamp: "2023-06-06T06:14:52Z"
//   name: dosecret
//   namespace: default
//   resourceVersion: "51339"
//   uid: d70b368d-2f10-4f93-ba37-d2659c45b6b3
// type: Opaque

import (
	"context"
	"fmt"
	"strings"

	"github.com/VedRatan/kluster/pkg/apis/vedratan.dev/v1alpha1"
	"github.com/digitalocean/godo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Create(c kubernetes.Interface, spec v1alpha1.KlusterSpec) (string, error){

	// we have to retreive the value from the key token as defined in the secret in k8s
	token, err := getToken(c, spec.TokenSecret)
	if err != nil{
		return "", err	
	}
	client := godo.NewFromToken(token)

	request := &godo.KubernetesClusterCreateRequest{
		Name:        spec.Name,
		RegionSlug:  spec.Region,
		VersionSlug: spec.Version,
		NodePools: []*godo.KubernetesNodePoolCreateRequest{
			{
				Size:  spec.NodePools[0].Size,
				Name:  spec.NodePools[0].Name,
				Count: spec.NodePools[0].Count,
			},
		},
	}

	cluster, _, err := client.Kubernetes.Create(context.Background(), request)
	if err != nil {
		return "", err
	}

	fmt.Println(client)

	return cluster.ID, nil
}

func ClusterState(c kubernetes.Interface, spec v1alpha1.KlusterSpec, id string) (string, error) {
	token, err := getToken(c, spec.TokenSecret)
	if err != nil {
		return "", err
	}

	client := godo.NewFromToken(token)

	cluster, _, err := client.Kubernetes.Get(context.Background(), id)
	return string(cluster.Status.State), err
}


func getToken(c kubernetes.Interface, sec string) (string, error) {
	namespace := strings.Split(sec, "/")[0]
	name :=  strings.Split(namespace, "/")[1]
	s, err := c.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(s.Data["token"]),nil      // see the above reference of secret to check how to access the value of the key token in secret 

}
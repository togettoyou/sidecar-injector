package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"path/filepath"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func createMutatingWebhookConfiguration(caPEM *bytes.Buffer) error {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	mutatingWebhookConfigV1Client := clientset.AdmissionregistrationV1()
	metaName := "sidecar-go-mutating-webhook-configuration"
	url := fmt.Sprintf("https://%s:%d/inject", hostname, port)

	mutatingWebhookConfig := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: metaName,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name:                    "namespace.sidecar-injector.togettoyou.com",
			AdmissionReviewVersions: []string{"v1"},
			SideEffects: func() *admissionregistrationv1.SideEffectClass {
				se := admissionregistrationv1.SideEffectClassNone
				return &se
			}(),
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caPEM.Bytes(),
				URL:      &url,
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
					},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
				},
			},
			FailurePolicy: func() *admissionregistrationv1.FailurePolicyType {
				pt := admissionregistrationv1.Fail
				return &pt
			}(),
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "sidecar-injector",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"enabled",
						},
					},
				},
			},
		}},
	}

	mutatingWebhookConfigV1Client.MutatingWebhookConfigurations().
		Delete(context.Background(), metaName, metav1.DeleteOptions{})
	_, err = mutatingWebhookConfigV1Client.MutatingWebhookConfigurations().
		Create(context.Background(), mutatingWebhookConfig, metav1.CreateOptions{})
	return err
}

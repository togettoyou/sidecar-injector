package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// 注入逻辑
func inject(w http.ResponseWriter, r *http.Request) {
	log.Println("收到请求")
	// 1.获取 body
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		http.Error(w, "no body found", http.StatusBadRequest)
		return
	}

	// 2.校验 content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "invalid Content-Type, want `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	// 3.解析 body 为 k8s pod 对象
	deserializer := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	ar := admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		http.Error(w, fmt.Sprintf("could not decode body: %v", err), http.StatusInternalServerError)
		return
	}
	var pod corev1.Pod
	if err := json.Unmarshal(ar.Request.Object.Raw, &pod); err != nil {
		http.Error(w, fmt.Sprintf("could not decode pod: %v", err), http.StatusInternalServerError)
		return
	}

	// 4.根据 sidecar 模板篡改资源，得到修改后的补丁
	sidecarTemp := []corev1.Container{
		{
			Name:    "sidecar",
			Image:   "busybox:1.28.4",
			Command: []string{"/bin/sh", "-c", "echo 'sidecar' && sleep 3600"},
		},
	}
	patch := addContainer(pod.Spec.Containers, sidecarTemp)
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not encode patch: %v", err), http.StatusInternalServerError)
		return
	}

	// 5.将篡改后的补丁内容写入 response
	admissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:     ar.Request.UID,
			Allowed: true,
			Patch:   patchBytes,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
		},
	}
	resp, err := json.Marshal(admissionReview)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(resp); err != nil {
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}

	log.Println("注入成功")
}

func addContainer(target, added []corev1.Container) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := "/spec/containers"
		if first {
			first = false
			value = []corev1.Container{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

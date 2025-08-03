/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/codewithmitesh/cisbenchk8.git/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CISBenchmarkReconciler reconciles a CISBenchmark object
type CISBenchmarkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=security.miteshtank.com,resources=cisbenchmarks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.miteshtank.com,resources=cisbenchmarks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=security.miteshtank.com,resources=cisbenchmarks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CISBenchmark object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *CISBenchmarkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the CISBenchmark instance
	instance := &securityv1alpha1.CISBenchmark{}

	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("CISBenchmark instance found", "webhook", instance.Spec.SlackWebhook)
	// Create a DaemonSet to run kube-bench on each node
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-bench",
			Namespace: req.Namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "kube-bench"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "kube-bench"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kube-bench",
							Image: "aquasec/kube-bench:latest",
							Command: []string{
								"sh", "-c",
								`
								  apk add --no-cache wget >/dev/null &&
								  kube-bench --json > /tmp/result.json &&
								  payload=$(cat /tmp/result.json | jq -Rs '{text: .}') &&
								  echo "$payload" > /tmp/slack.json &&
								  wget --header='Content-Type: application/json' --method=POST --body-file=/tmp/slack.json ` + instance.Spec.SlackWebhook,
							},

							VolumeMounts: []corev1.VolumeMount{
								{Name: "output", MountPath: "/output"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "output",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways},
			},
		},
	}

	// Apply the DaemonSet
	if err := r.Create(ctx, ds); err != nil {
		return ctrl.Result{}, client.IgnoreAlreadyExists(err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CISBenchmarkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.CISBenchmark{}).
		Named("cisbenchmark").
		Complete(r)
}

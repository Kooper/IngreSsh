/*
Copyright 2023.

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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ing "kuberstein.io/ingressh/api/v1"
	"kuberstein.io/ingressh/internal/server"
	"kuberstein.io/ingressh/internal/types"
)

// IngreSshReconciler reconciles a IngreSsh object
type IngreSshReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// finalizerName is the name of the custom finalizer to handle the resource deletion
const finalizerName = "ingressh.ingress.kuberstein.io/finalizer"

//+kubebuilder:rbac:groups=ingress.kuberstein.io,resources=ingresshes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ingress.kuberstein.io,resources=ingresshes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ingress.kuberstein.io,resources=ingresshes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the IngreSsh object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile

// The job is to:
// - Load the named IngreSsh
// - Configure the SSH server accordingly
// - Get list of active sessions and update the status
func (r *IngreSshReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	ingreSsh := &ing.IngreSsh{}

	// FIXME delete the resource. How to get what was deleted?
	if err := r.Get(ctx, req.NamespacedName, ingreSsh); err != nil {
		log.Error(err, "unable to fetch IngreSsh resource, return IgnoreNotFound")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Init ssh configuration object from the ingress object
	sshConfig := &types.SshConfig{
		IngreSshSpec: *&ingreSsh.Spec,
		Namespace:    req.Namespace,
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if ingreSsh.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted. Register finalizer to handle
		// the future deletion.
		if !controllerutil.ContainsFinalizer(ingreSsh, finalizerName) {
			controllerutil.AddFinalizer(ingreSsh, finalizerName)
			if err := r.Update(ctx, ingreSsh); err != nil {
				return ctrl.Result{}, err
			}
		}

	} else {

		// The object is being deleted
		log.Info("ingreSsh resource deletion")

		if controllerutil.ContainsFinalizer(ingreSsh, finalizerName) {
			log.Info("delete routes from the SSH server accordingly to resource configuration")
			log.Info(fmt.Sprintf("sshConfig: %v", sshConfig))
			// The finalizer is present, handle external dependency and
			// remove the finalizer.
			server.Routes.Delete(sshConfig)
			controllerutil.RemoveFinalizer(ingreSsh, finalizerName)
			if err := r.Update(ctx, ingreSsh); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// TODO: How to set properties of the exactly this configuration object?
	// Should I remove the previous object and register the new one? How to
	// find out what's the new one and what was the old one? Is there a name
	// of the resource?
	log.Info("configure SSH server accordingly to resource configuration")
	log.Info(fmt.Sprintf("sshConfig: %v", sshConfig))
	server.Routes.Set(sshConfig)

	// TODO Update status?

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngreSshReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ing.IngreSsh{}).
		Complete(r)
}

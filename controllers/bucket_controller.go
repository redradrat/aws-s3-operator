/*


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

package controllers

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/predicate"

	awssdk "github.com/aws/aws-sdk-go/aws"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	awss3 "github.com/aws/aws-sdk-go/service/s3"
	awss3v1beta1 "github.com/redradrat/aws-s3-operator/api/v1beta1"
	"github.com/redradrat/cloud-objects/aws/s3"
)

// BucketReconciler reconciles a Bucket object
type BucketReconciler struct {
	client.Client
	Region string
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	BucketDeletionFinalizer = "aws-s3.redradrat.xyz/delete-bucket"
)

// +kubebuilder:rbac:groups=aws-s3.redradrat.xyz,resources=buckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-s3.redradrat.xyz,resources=buckets/status,verbs=get;update;patch

func (r *BucketReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("bucket", req.NamespacedName)

	log.Info(fmt.Sprintf("reconciliation triggered for '%s'", req.NamespacedName))

	obj := &awss3v1beta1.Bucket{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	sess, err := getSession(awssdk.String(r.Region))
	if err != nil {
		return ctrl.Result{}, err
	}

	// New Bucket Cloud Object with our k8s resource name
	bkt, err := s3.NewBucket(fmt.Sprintf("%s-%s", req.Name, req.Namespace), sess)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Deletion?
	if !obj.GetDeletionTimestamp().IsZero() {
		// Deletion.

		if controllerutil.ContainsFinalizer(obj, BucketDeletionFinalizer) {

			// Delete the bucket
			if err := bkt.Delete(false); err != nil {
				return ctrl.Result{}, err
			}

			// Finally remove the finalizer
			controllerutil.RemoveFinalizer(obj, BucketDeletionFinalizer)

			if err := r.Update(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
		}

	} else {
		// Not deletion.

		// Add Finalizer if not present
		if !controllerutil.ContainsFinalizer(obj, BucketDeletionFinalizer) {
			controllerutil.AddFinalizer(obj, BucketDeletionFinalizer)

			if err := r.Update(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
		}

		spec := &s3.BucketSpec{
			ACL:                   awss3.BucketCannedACLPrivate,
			ObjectLock:            obj.Spec.ObjectLock,
			Versioning:            obj.Spec.Versioning,
			TransferAcceleration:  obj.Spec.TransferAcceleration,
			BlockPublicAcls:       obj.Spec.BlockPublicACLs,
			IgnorePublicAcls:      obj.Spec.IgnorePublicACLs,
			BlockPublicPolicy:     obj.Spec.BlockPublicPolicy,
			RestrictPublicBuckets: obj.Spec.RestrictPublicBuckets,
		}

		if !obj.Status.Initialized {
			// Call Create() on the cloudobject if it isn't yet initialized
			_, err := bkt.Create(spec)
			if err != nil {
				return ctrl.Result{}, err
			}

			obj.Status.Initialized = true
			if err := r.Status().Update(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			// Call Update() on the cloudobject if it is already initialized
			_, err := bkt.Update(spec)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awss3v1beta1.Bucket{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func getSession(region *string) (awsclient.ConfigProvider, error) {
	conf := &awssdk.Config{}
	if region != nil {
		conf.Region = region
	}

	cp, err := session.NewSession(conf)
	if err != nil {
		return nil, err
	}

	return cp, nil
}

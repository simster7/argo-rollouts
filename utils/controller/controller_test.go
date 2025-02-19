package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"

	"github.com/argoproj/argo-rollouts/controller/metrics"
	register "github.com/argoproj/argo-rollouts/pkg/apis/rollouts"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/fake"
	informers "github.com/argoproj/argo-rollouts/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-rollouts/utils/log"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

func TestProcessNextWorkItemShutDownQueue(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	syncHandler := func(key string) error {
		return nil
	}
	q.ShutDown()
	assert.False(t, processNextWorkItem(q, log.RolloutKey, syncHandler, nil))
}

func TestProcessNextWorkItemNoTStringKey(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	q.Add(1)
	syncHandler := func(key string) error {
		return nil
	}
	assert.True(t, processNextWorkItem(q, log.RolloutKey, syncHandler, nil))
}

func TestProcessNextWorkItemNoValidKey(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	q.Add("invalid.key")
	syncHandler := func(key string) error {
		return nil
	}
	assert.True(t, processNextWorkItem(q, log.RolloutKey, syncHandler, nil))
}

func TestProcessNextWorkItemNormalSync(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	q.Add("valid/key")
	syncHandler := func(key string) error {
		return nil
	}
	assert.True(t, processNextWorkItem(q, log.RolloutKey, syncHandler, nil))
}

func TestProcessNextWorkItemSyncHandlerReturnError(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	q.Add("valid/key")
	metricServer := metrics.NewMetricsServer("localhost:8080", nil)
	syncHandler := func(key string) error {
		return fmt.Errorf("error message")
	}
	assert.True(t, processNextWorkItem(q, log.RolloutKey, syncHandler, metricServer))
}

func TestEnqueue(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	r := &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "testNamespace",
		},
	}
	Enqueue(r, q)
	assert.Equal(t, 1, q.Len())
}

func TestEnqueueInvalidObj(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	Enqueue("Invalid Object", q)
	assert.Equal(t, 0, q.Len())
}

func TestEnqueueAfter(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	r := &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "testNamespace",
		},
	}
	EnqueueAfter(r, time.Duration(1), q)
	assert.Equal(t, 0, q.Len())
	time.Sleep(2 * time.Second)
	assert.Equal(t, 1, q.Len())
}

func TestEnqueueAfterInvalidObj(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	EnqueueAfter("Invalid Object", time.Duration(1), q)
	assert.Equal(t, 0, q.Len())
	time.Sleep(2 * time.Second)
	assert.Equal(t, 0, q.Len())
}

func TestEnqueueRateLimited(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	r := &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "testNamespace",
		},
	}
	EnqueueRateLimited(r, q)
	assert.Equal(t, 0, q.Len())
	time.Sleep(time.Second)
	assert.Equal(t, 1, q.Len())
}

func TestEnqueueRateLimitedInvalidObject(t *testing.T) {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rollouts")
	EnqueueRateLimited("invalid Object", q)
	assert.Equal(t, 0, q.Len())
	time.Sleep(time.Second)
	assert.Equal(t, 0, q.Len())
}

func TestEnqueueParentObjectInvalidObject(t *testing.T) {
	errorMessages := make([]error, 0)
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorMessages = append(errorMessages, err)
	})
	invalidObject := "invalid-object"
	enqueueFunc := func(obj interface{}) {}
	EnqueueParentObject(invalidObject, register.RolloutKind, nil, enqueueFunc)
	assert.Len(t, errorMessages, 1)
	assert.Error(t, errorMessages[0], "error decoding object, invalid type")
}

func TestEnqueueParentObjectInvalidTombstoneObject(t *testing.T) {
	errorMessages := make([]string, 0)
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorMessages = append(errorMessages, err.Error())
	})

	invalidObject := cache.DeletedFinalStateUnknown{}
	enqueueFunc := func(obj interface{}) {}
	EnqueueParentObject(invalidObject, register.RolloutKind, nil, enqueueFunc)
	assert.Len(t, errorMessages, 1)
	assert.Equal(t, "error decoding object tombstone, invalid type", errorMessages[0])
}

func TestEnqueueParentObjectNoOwner(t *testing.T) {
	errorMessages := make([]string, 0)
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorMessages = append(errorMessages, err.Error())
	})
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs",
			Namespace: "default",
		},
	}
	enqueuedObjs := make([]interface{}, 0)
	enqueueFunc := func(obj interface{}) {
		enqueuedObjs = append(enqueuedObjs, obj)
	}
	EnqueueParentObject(rs, register.RolloutKind, nil, enqueueFunc)
	assert.Len(t, errorMessages, 0)
	assert.Len(t, enqueuedObjs, 0)
}

func TestEnqueueParentObjectDifferentOwnerKind(t *testing.T) {
	experimentKind := v1alpha1.SchemeGroupVersion.WithKind("Experiment")

	errorMessages := make([]string, 0)
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorMessages = append(errorMessages, err.Error())
	})
	experiment := &v1alpha1.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ex",
			Namespace: "default",
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "rs",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(experiment, experimentKind)},
		},
	}
	enqueuedObjs := make([]interface{}, 0)
	enqueueFunc := func(obj interface{}) {
		enqueuedObjs = append(enqueuedObjs, obj)
	}
	EnqueueParentObject(rs, register.RolloutKind, nil, enqueueFunc)
	assert.Len(t, errorMessages, 0)
	assert.Len(t, enqueuedObjs, 0)
}

func TestEnqueueParentObjectPanicNonValidOwnerType(t *testing.T) {
	deploymentKind := appsv1.SchemeGroupVersion.WithKind("Deployment")

	errorMessages := make([]string, 0)
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorMessages = append(errorMessages, err.Error())
	})
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ex",
			Namespace: "default",
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "rs",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(deployment, deploymentKind)},
		},
	}
	enqueuedObjs := make([]interface{}, 0)
	enqueueFunc := func(obj interface{}) {
		enqueuedObjs = append(enqueuedObjs, obj)
	}
	panicFunc := func() {
		EnqueueParentObject(rs, "Deployment", nil, enqueueFunc)
	}
	assert.Panics(t, panicFunc, "OwnerType of parent is not a Rollout or a Experiment")
	assert.Len(t, errorMessages, 0)
	assert.Len(t, enqueuedObjs, 0)
}

func TestEnqueueParentObjectEnqueueExperiment(t *testing.T) {
	experimentKind := v1alpha1.SchemeGroupVersion.WithKind("Experiment")

	errorMessages := make([]string, 0)
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorMessages = append(errorMessages, err.Error())
	})
	experiment := &v1alpha1.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ex",
			Namespace: "default",
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "rs",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(experiment, experimentKind)},
		},
	}
	enqueuedObjs := make([]interface{}, 0)
	enqueueFunc := func(obj interface{}) {
		enqueuedObjs = append(enqueuedObjs, obj)
	}
	client := fake.NewSimpleClientset(experiment)
	i := informers.NewSharedInformerFactory(client, 0)
	i.Argoproj().V1alpha1().Experiments().Informer().GetIndexer().Add(experiment)

	lister := i.Argoproj().V1alpha1().Experiments().Lister()
	EnqueueParentObject(rs, register.ExperimentKind, lister, enqueueFunc)
	assert.Len(t, errorMessages, 0)
	assert.Len(t, enqueuedObjs, 1)
}

func TestEnqueueParentObjectEnqueueRollout(t *testing.T) {
	rolloutKind := v1alpha1.SchemeGroupVersion.WithKind("Rollout")

	errorMessages := make([]string, 0)
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorMessages = append(errorMessages, err.Error())
	})
	rollout := &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ex",
			Namespace: "default",
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "rs",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(rollout, rolloutKind)},
		},
	}
	enqueuedObjs := make([]interface{}, 0)
	enqueueFunc := func(obj interface{}) {
		enqueuedObjs = append(enqueuedObjs, obj)
	}
	client := fake.NewSimpleClientset(rollout)
	i := informers.NewSharedInformerFactory(client, 0)
	i.Argoproj().V1alpha1().Rollouts().Informer().GetIndexer().Add(rollout)

	lister := i.Argoproj().V1alpha1().Rollouts().Lister()
	EnqueueParentObject(rs, register.RolloutKind, lister, enqueueFunc)
	assert.Len(t, errorMessages, 0)
	assert.Len(t, enqueuedObjs, 1)
}

func TestEnqueueParentListerError(t *testing.T) {
	rolloutKind := v1alpha1.SchemeGroupVersion.WithKind("Rollout")

	errorMessages := make([]string, 0)
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorMessages = append(errorMessages, err.Error())
	})
	rollout := &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rollout",
			Namespace: "default",
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "rs",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(rollout, rolloutKind)},
		},
	}
	enqueuedObjs := make([]interface{}, 0)
	enqueueFunc := func(obj interface{}) {
		enqueuedObjs = append(enqueuedObjs, obj)
	}
	client := fake.NewSimpleClientset(rollout)
	i := informers.NewSharedInformerFactory(client, 0)
	lister := i.Argoproj().V1alpha1().Rollouts().Lister()
	EnqueueParentObject(rs, register.RolloutKind, lister, enqueueFunc)
	assert.Len(t, errorMessages, 0)
	assert.Len(t, enqueuedObjs, 0)
}

func TestEnqueueParentObjectRecoverTombstoneObject(t *testing.T) {
	experimentKind := v1alpha1.SchemeGroupVersion.WithKind("Experiment")
	errorMessages := make([]string, 0)
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorMessages = append(errorMessages, err.Error())
	})
	experiment := &v1alpha1.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ex",
			Namespace: "default",
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "rs",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(experiment, experimentKind)},
		},
	}
	invalidObject := cache.DeletedFinalStateUnknown{
		Key: "default/rs",
		Obj: rs,
	}

	enqueuedObjs := make([]interface{}, 0)
	enqueueFunc := func(obj interface{}) {
		enqueuedObjs = append(enqueuedObjs, obj)
	}
	client := fake.NewSimpleClientset(experiment)
	i := informers.NewSharedInformerFactory(client, 0)
	i.Argoproj().V1alpha1().Experiments().Informer().GetIndexer().Add(experiment)

	lister := i.Argoproj().V1alpha1().Experiments().Lister()
	EnqueueParentObject(invalidObject, register.ExperimentKind, lister, enqueueFunc)
	assert.Len(t, errorMessages, 0)
	assert.Len(t, enqueuedObjs, 1)
}

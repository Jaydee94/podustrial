package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/Jaydee94/podustrial/internal/factory"
)

type Watcher struct {
	client *Client
	out    chan<- factory.Event
}

func NewWatcher(client *Client, out chan<- factory.Event) *Watcher {
	return &Watcher{client: client, out: out}
}

func (w *Watcher) Run(ctx context.Context) error {
	factoryInformers := informers.NewSharedInformerFactoryWithOptions(
		w.client.Clientset,
		0,
		informers.WithNamespace(w.client.namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = ManagedByLabel + "=" + ManagedByValue
		}),
	)
	podInformer := factoryInformers.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if ok {
				w.out <- factory.PodEventToFactoryEvent(factory.EventMachineAdded, pod)
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			pod, ok := newObj.(*corev1.Pod)
			if ok {
				w.out <- factory.PodEventToFactoryEvent(factory.EventMachineUpdated, pod)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				pod, ok = tombstone.Obj.(*corev1.Pod)
				if !ok {
					return
				}
			}
			w.out <- factory.PodEventToFactoryEvent(factory.EventMachineRemoved, pod)
		},
	})

	factoryInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
		return context.Canceled
	}
	w.out <- factory.NewClusterStatusEvent(factory.ClusterStatusOK)
	<-ctx.Done()
	return ctx.Err()
}

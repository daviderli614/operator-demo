// controllers/resource.go

package controllers

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/daviderli614/kubebuilder-demo/api/v1"
)

var (
	ElasticWebCommonLabelKey = "app"
)

const (
	// APP_NAME is deployment label
	APP_NAME = "elastic-app"

	CONTAINER_PORT = 80

	CPU_REQUEST = "100m"

	CPU_LIMIT = "100m"

	MEM_REQUEST = "100Mi"

	MEM_LIMIT = "100Mi"
)

func getExpectReplicas(elasticWeb *v1.ElasticWeb) int32 {
	singlePodQPS := *elasticWeb.Spec.SinglePodsQPS
	totalQPS := *elasticWeb.Spec.TotalQPS
	replicas := totalQPS / singlePodQPS

	if totalQPS%singlePodQPS != 0 {
		replicas += 1
	}
	return replicas
}

func CreateServiceIfNotExists(ctx context.Context, r *ElasticWebReconciler, elasticWeb *v1.ElasticWeb, req ctrl.Request) error {
	logger := log.FromContext(ctx)
	logger.WithValues("func", "createService")
	svc := &corev1.Service{}

	svc.Name = elasticWeb.Name
	svc.Namespace = elasticWeb.Namespace

	svc.Spec = corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:     "http",
				Port:     CONTAINER_PORT,
				NodePort: *elasticWeb.Spec.Port,
			},
		},
		Type: corev1.ServiceTypeNodePort,
		Selector: map[string]string{
			ElasticWebCommonLabelKey: APP_NAME,
		},
	}

	logger.Info("set reference")
	if err := controllerutil.SetControllerReference(elasticWeb, svc, r.Scheme); err != nil {
		logger.Error(err, "SetControllerReference error")
		return err
	}

	logger.Info("start create service")
	if err := r.Create(ctx, svc); err != nil {
		logger.Error(err, "create service error")
		return err
	}

	return nil
}

func CreateDeployment(ctx context.Context, r *ElasticWebReconciler, elasticWeb *v1.ElasticWeb) error {
	logger := log.FromContext(ctx)
	logger.WithValues("func", "createDeploy")

	expectReplicas := getExpectReplicas(elasticWeb)
	logger.Info(fmt.Sprintf("expectReplicas [%d]", expectReplicas))

	deploy := &appsv1.Deployment{}

	deploy.Labels = map[string]string{
		ElasticWebCommonLabelKey: APP_NAME,
	}

	deploy.Name = elasticWeb.Name
	deploy.Namespace = elasticWeb.Namespace

	deploy.Spec = appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(expectReplicas),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				ElasticWebCommonLabelKey: APP_NAME,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					ElasticWebCommonLabelKey: APP_NAME,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  APP_NAME,
						Image: elasticWeb.Spec.Image,
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: CONTAINER_PORT,
								Protocol:      corev1.ProtocolSCTP,
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(CPU_LIMIT),
								corev1.ResourceMemory: resource.MustParse(MEM_LIMIT),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(CPU_REQUEST),
								corev1.ResourceMemory: resource.MustParse(MEM_REQUEST),
							},
						},
					},
				},
			},
		},
	}

	logger.Info("set reference")
	if err := controllerutil.SetControllerReference(elasticWeb, deploy, r.Scheme); err != nil {
		logger.Error(err, "SetControllerReference error")
		return err
	}

	logger.Info("start create deploy")
	if err := r.Create(ctx, deploy); err != nil {
		logger.Error(err, "create deploy error")
		return err
	}

	logger.Info("create deploy success")
	return nil
}

func UpdateStatus(ctx context.Context, r *ElasticWebReconciler, elasticWeb *v1.ElasticWeb) error {
	logger := log.FromContext(ctx)
	logger.WithValues("func", "updateStatus")

	singlePodQPS := *elasticWeb.Spec.SinglePodsQPS

	replicas := getExpectReplicas(elasticWeb)

	if nil == elasticWeb.Status.RealQPS {
		elasticWeb.Status.RealQPS = new(int32)
	}

	*elasticWeb.Status.RealQPS = singlePodQPS * replicas
	logger.Info(fmt.Sprintf("singlePodQPS [%d],replicas [%d],realQPS[%d]", singlePodQPS, replicas, *elasticWeb.Status.RealQPS))

	if err := r.Update(ctx, elasticWeb); err != nil {
		logger.Error(err, "update instance error")
		return err
	}
	return nil
}

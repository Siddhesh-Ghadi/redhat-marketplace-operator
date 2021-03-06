// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:generate go-bindata -o bindata.go -prefix "../../" -pkg manifests ../../assets/...

package manifests

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/gotidy/ptr"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	PrometheusOperatorDeployment    = "assets/prometheus-operator/deployment.yaml"
	PrometheusOperatorService       = "assets/prometheus-operator/service.yaml"
	PrometheusOperatorCertsCABundle = "assets/prometheus-operator/operator-certs-ca-bundle.yaml"

	PrometheusAdditionalScrapeConfig = "assets/prometheus/additional-scrape-configs.yaml"
	PrometheusHtpasswd               = "assets/prometheus/htpasswd-secret.yaml"
	PrometheusRBACProxySecret        = "assets/prometheus/kube-rbac-proxy-secret.yaml"
	PrometheusDeployment             = "assets/prometheus/prometheus.yaml"
	PrometheusProxySecret            = "assets/prometheus/proxy-secret.yaml"
	PrometheusService                = "assets/prometheus/service.yaml"
	PrometheusDatasourcesSecret      = "assets/prometheus/prometheus-datasources-secret.yaml"
	PrometheusServingCertsCABundle   = "assets/prometheus/serving-certs-ca-bundle.yaml"
	PrometheusKubeletServingCABundle = "assets/prometheus/kubelet-serving-ca-bundle.yaml"

	ReporterJob = "assets/reporter/job.yaml"

	MetricStateDeployment     = "assets/metric-state/deployment.yaml"
	MetricStateServiceMonitor = "assets/metric-state/service-monitor.yaml"
	MetricStateService        = "assets/metric-state/service.yaml"
)

var log = logf.Log.WithName("manifests_factory")

func MustAssetReader(asset string) io.Reader {
	return bytes.NewReader(MustAsset(asset))
}

type Factory struct {
	namespace string
	config    *Config
}

func NewFactory(namespace string, c *Config) *Factory {
	return &Factory{
		namespace: namespace,
		config:    c,
	}
}

func (f *Factory) ReplaceImages(container *corev1.Container) {
	switch {
	case strings.HasPrefix(container.Name, "kube-rbac-proxy"):
		container.Image = f.config.RelatedImages.KubeRbacProxy
	case container.Name == "metric-state":
		container.Image = f.config.RelatedImages.MetricState
	case container.Name == "authcheck":
		container.Image = f.config.RelatedImages.AuthChecker
	case container.Name == "prometheus-operator":
		container.Image = f.config.RelatedImages.PrometheusOperator
	case container.Name == "prometheus-proxy":
		container.Image = f.config.RelatedImages.OAuthProxy
	}
}

func (f *Factory) NewDeployment(manifest io.Reader) (*appsv1.Deployment, error) {
	d, err := NewDeployment(manifest)
	if err != nil {
		return nil, err
	}

	if d.GetNamespace() == "" {
		d.SetNamespace(f.namespace)
	}

	return d, nil
}

func (f *Factory) NewService(manifest io.Reader) (*corev1.Service, error) {
	d, err := NewService(manifest)
	if err != nil {
		return nil, err
	}

	if d.GetNamespace() == "" {
		d.SetNamespace(f.namespace)
	}

	return d, nil
}

func (f *Factory) NewConfigMap(manifest io.Reader) (*corev1.ConfigMap, error) {
	d, err := NewConfigMap(manifest)
	if err != nil {
		return nil, err
	}

	if d.GetNamespace() == "" {
		d.SetNamespace(f.namespace)
	}

	return d, nil
}

func (f *Factory) NewSecret(manifest io.Reader) (*v1.Secret, error) {
	s, err := NewSecret(manifest)
	if err != nil {
		return nil, err
	}

	if s.GetNamespace() == "" {
		s.SetNamespace(f.namespace)
	}

	return s, nil
}

func (f *Factory) NewJob(manifest io.Reader) (*batchv1.Job, error) {
	j, err := NewJob(manifest)
	if err != nil {
		return nil, err
	}

	if j.GetNamespace() == "" {
		j.SetNamespace(f.namespace)
	}

	return j, nil
}

func (f *Factory) NewPrometheus(
	manifest io.Reader,
) (*monitoringv1.Prometheus, error) {
	p, err := NewPrometheus(manifest)
	if err != nil {
		return nil, err
	}

	if p.GetNamespace() == "" {
		p.SetNamespace(f.namespace)
	}

	return p, nil
}

func (f *Factory) PrometheusService(instanceName string) (*v1.Service, error) {
	s, err := f.NewService(MustAssetReader(PrometheusService))
	if err != nil {
		return nil, err
	}

	s.Namespace = f.namespace

	s.Labels["app"] = "prometheus"
	s.Labels["prometheus"] = instanceName

	s.Spec.Selector["prometheus"] = instanceName

	return s, nil
}

func (f *Factory) PrometheusRBACProxySecret() (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusRBACProxySecret))
	if err != nil {
		return nil, err
	}

	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) PrometheusProxySecret() (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusProxySecret))
	if err != nil {
		return nil, err
	}

	p, err := GeneratePassword(43)
	if err != nil {
		return nil, err
	}
	s.Data["session_secret"] = []byte(p)
	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) PrometheusAdditionalConfigSecret(data []byte) (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusAdditionalScrapeConfig))
	if err != nil {
		return nil, err
	}

	s.Data["meterdef.yaml"] = data
	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) NewPrometheusOperatorDeployment(ns []string) (*appsv1.Deployment, error) {
	c := f.config.PrometheusOperatorConfig
	dep, err := f.NewDeployment(MustAssetReader(PrometheusOperatorDeployment))

	if len(c.NodeSelector) > 0 {
		dep.Spec.Template.Spec.NodeSelector = c.NodeSelector
	}

	if len(c.Tolerations) > 0 {
		dep.Spec.Template.Spec.Tolerations = c.Tolerations
	}

	if c.ServiceAccountName != "" {
		dep.Spec.Template.Spec.ServiceAccountName = c.ServiceAccountName
	}

	replacer := strings.NewReplacer(
		"{{NAMESPACE}}", f.namespace,
		"{{NAMESPACES}}", strings.Join(ns, ","),
		"{{CONFIGMAP_RELOADER_IMAGE}}", f.config.RelatedImages.ConfigMapReloader,
		"{{PROM_CONFIGMAP_RELOADER_IMAGE}}", f.config.RelatedImages.PrometheusConfigMapReloader,
	)

	for i := range dep.Spec.Template.Spec.Containers {
		container := &dep.Spec.Template.Spec.Containers[i]
		newArgs := []string{}

		for _, arg := range container.Args {
			newArg := replacer.Replace(arg)
			newArgs = append(newArgs, newArg)
		}

		f.ReplaceImages(container)
		container.Args = newArgs
	}

	return dep, err
}

func (f *Factory) NewPrometheusDeployment(
	cr *marketplacev1alpha1.MeterBase,
	cfg *corev1.Secret,
) (*monitoringv1.Prometheus, error) {
	logger := log.WithValues("func", "NewPrometheusDeployment")
	p, err := f.NewPrometheus(MustAssetReader(PrometheusDeployment))

	if err != nil {
		logger.Error(err, "failed to read the file")
		return p, err
	}

	p.Name = cr.Name
	p.ObjectMeta.Name = cr.Name

	p.Spec.Image = &f.config.RelatedImages.Prometheus

	if cr.Spec.Prometheus.Replicas != nil {
		p.Spec.Replicas = cr.Spec.Prometheus.Replicas
	}

	if f.config.PrometheusConfig.Retention != "" {
		p.Spec.Retention = f.config.PrometheusConfig.Retention
	}

	//Set empty dir if present in the CR, will override a pvc specified (per prometheus docs)
	if cr.Spec.Prometheus.Storage.EmptyDir != nil {
		p.Spec.Storage.EmptyDir = cr.Spec.Prometheus.Storage.EmptyDir
	}

	storageClass := ptr.String("")
	if cr.Spec.Prometheus.Storage.Class != nil {
		storageClass = cr.Spec.Prometheus.Storage.Class
	}

	pvc, err := utils.NewPersistentVolumeClaim(utils.PersistentVolume{
		ObjectMeta: &metav1.ObjectMeta{
			Name: "storage-volume",
		},
		StorageClass: storageClass,
		StorageSize:  &cr.Spec.Prometheus.Storage.Size,
	})

	p.Spec.Storage.VolumeClaimTemplate = monitoringv1.EmbeddedPersistentVolumeClaim{
		Spec: pvc.Spec,
	}

	if cfg != nil {
		p.Spec.AdditionalScrapeConfigs = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: cfg.GetName(),
			},
			Key: "meterdef.yaml",
		}
	}

	for i := range p.Spec.Containers {
		f.ReplaceImages(&p.Spec.Containers[i])
	}

	return p, err
}

func (f *Factory) NewPrometheusOperatorService() (*corev1.Service, error) {
	service, err := f.NewService(MustAssetReader(PrometheusOperatorService))

	return service, err
}

func (f *Factory) NewPrometheusOperatorCertsCABundle() (*corev1.ConfigMap, error) {
	return f.NewConfigMap(MustAssetReader(PrometheusOperatorCertsCABundle))
}

func (f *Factory) PrometheusKubeletServingCABundle(data map[string]string) (*v1.ConfigMap, error) {
	c, err := f.NewConfigMap(MustAssetReader(PrometheusKubeletServingCABundle))
	if err != nil {
		return nil, err
	}

	c.Namespace = f.namespace
	c.Data = data

	return c, nil
}

func (f *Factory) PrometheusDatasources() (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusDatasourcesSecret))
	if err != nil {
		return nil, err
	}

	secret, err := GeneratePassword(255)

	if err != nil {
		return nil, err
	}

	if s.Data == nil {
		s.Data = make(map[string][]byte)
	}

	s.Data["basicAuthSecret"] = []byte(secret)

	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) PrometheusHtpasswdSecret(password string) (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusHtpasswd))
	if err != nil {
		return nil, err
	}

	f.generateHtpasswdSecret(s, password)
	return s, nil
}

func (f *Factory) generateHtpasswdSecret(s *v1.Secret, password string) {
	h := sha1.New()
	h.Write([]byte(password))
	s.Data["auth"] = []byte("internal:{SHA}" + base64.StdEncoding.EncodeToString(h.Sum(nil)))
	s.Namespace = f.namespace
}

func (f *Factory) PrometheusServingCertsCABundle() (*v1.ConfigMap, error) {
	c, err := f.NewConfigMap(MustAssetReader(PrometheusServingCertsCABundle))
	if err != nil {
		return nil, err
	}

	c.Namespace = f.namespace

	return c, nil
}

func (f *Factory) ReporterJob(report *marketplacev1alpha1.MeterReport) (*batchv1.Job, error) {
	j, err := f.NewJob(MustAssetReader(ReporterJob))

	if err != nil {
		return nil, err
	}

	container := j.Spec.Template.Spec.Containers[0]
	container.Image = f.config.RelatedImages.Reporter

	j.Name = report.GetName()
	container.Args = append(container.Args,
		"--name",
		report.Name,
		"--namespace",
		report.Namespace,
	)

	if len(report.Spec.ExtraArgs) > 0 {
		container.Args = append(container.Args, report.Spec.ExtraArgs...)
	}

	j.Spec.Template.Spec.Containers[0] = container

	return j, nil
}

func (f *Factory) MetricStateDeployment() (*appsv1.Deployment, error) {
	d, err := f.NewDeployment(MustAssetReader(MetricStateDeployment))
	if err != nil {
		return nil, err
	}

	for i := range d.Spec.Template.Spec.Containers {
		f.ReplaceImages(&d.Spec.Template.Spec.Containers[i])
	}

	d.Namespace = f.namespace

	return d, nil
}

func (f *Factory) MetricStateServiceMonitor() (*monitoringv1.ServiceMonitor, error) {
	sm, err := f.NewServiceMonitor(MustAssetReader(MetricStateServiceMonitor))
	if err != nil {
		return nil, err
	}

	sm.Spec.Endpoints[0].TLSConfig.ServerName = fmt.Sprintf("rhm-metric-state-service.%s.svc", f.namespace)
	sm.Spec.Endpoints[1].TLSConfig.ServerName = fmt.Sprintf("rhm-metric-state-service.%s.svc", f.namespace)
	sm.Namespace = f.namespace

	return sm, nil
}

func (f *Factory) MetricStateService() (*v1.Service, error) {
	s, err := f.NewService(MustAssetReader(MetricStateService))
	if err != nil {
		return nil, err
	}

	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) NewServiceMonitor(manifest io.Reader) (*monitoringv1.ServiceMonitor, error) {
	sm, err := NewServiceMonitor(manifest)
	if err != nil {
		return nil, err
	}

	if sm.GetNamespace() == "" {
		sm.SetNamespace(f.namespace)
	}

	return sm, nil
}

func NewDeployment(manifest io.Reader) (*appsv1.Deployment, error) {
	d := appsv1.Deployment{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&d)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func NewConfigMap(manifest io.Reader) (*v1.ConfigMap, error) {
	cm := v1.ConfigMap{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&cm)
	if err != nil {
		return nil, err
	}

	return &cm, nil
}

func NewService(manifest io.Reader) (*v1.Service, error) {
	s := v1.Service{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&s)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func NewPrometheus(manifest io.Reader) (*monitoringv1.Prometheus, error) {
	s := monitoringv1.Prometheus{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&s)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func NewSecret(manifest io.Reader) (*v1.Secret, error) {
	s := v1.Secret{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func NewJob(manifest io.Reader) (*batchv1.Job, error) {
	j := batchv1.Job{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&j)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// GeneratePassword returns a base64 encoded securely random bytes.
func GeneratePassword(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), err
}

func NewServiceMonitor(manifest io.Reader) (*monitoringv1.ServiceMonitor, error) {
	sm := monitoringv1.ServiceMonitor{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&sm)
	if err != nil {
		return nil, err
	}

	return &sm, nil
}

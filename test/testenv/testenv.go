package testenv

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func SetupTestEnv(
	log logr.Logger,
	cfg *rest.Config,
	k8sClient client.Client,
	k8sManager manager.Manager,
	testEnv *envtest.Environment,
	namespaceName string,
	done Done,
) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	t := true
	if os.Getenv("TEST_USE_EXISTING_CLUSTER") == "true" {
		testEnv.UseExistingCluster = &t
	} else {
		testEnv.CRDDirectoryPaths = []string{
			filepath.Join("..", "..", "deploy", "crds"),
		}
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	connSchemes := initializeLocalSchemes()

	Expect(connSchemes).ToNot(BeEmpty())

	for _, conScheme := range connSchemes {
		err := conScheme.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())
	}

	// +kubebuilder:scaffold:scheme

	controllers := initializeControllers()

	opts := manager.Options{
		Namespace: "",
		Scheme:    scheme.Scheme,
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, opts)
	Expect(err).ToNot(HaveOccurred())

	for _, control := range controllers {
		err := control.Add(mgr)
		Expect(err).ToNot(HaveOccurred())
	}

	go func() {
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = mgr.GetClient()
	Expect(k8sClient).ToNot(BeNil())
	Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	})).To(Or(WithTransform(IsNotFound, BeTrue()), Succeed()))

	close(done)
}

func TeardownTestEnv(testEnv *envtest.Environment) {
	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
}

func IsNotFound(e error) bool { return apierrors.IsNotFound(e) }

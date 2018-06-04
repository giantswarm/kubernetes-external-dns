// +build k8srequired

package integration

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/giantswarm/e2e-harness/pkg/framework"
	"github.com/giantswarm/microerror"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	resourceNamespace = "kube-system"
)

var (
	f *framework.Host
)

// TestMain allows us to have common setup and teardown steps that are run
// once for all the tests https://golang.org/pkg/testing/#hdr-Main.
func TestMain(m *testing.M) {
	var v int
	var err error

	f, err = framework.NewHost(framework.HostConfig{})
	if err != nil {
		panic(err.Error())
	}

	if err := f.CreateNamespace("giantswarm"); err != nil {
		log.Printf("unexpected error: %v\n", err)
		v = 1
	}

	if v == 0 {
		v = m.Run()
	}

	if os.Getenv("KEEP_RESOURCES") != "true" {
		f.Teardown()
	}

	os.Exit(v)
}

func TestHelm(t *testing.T) {
	channel := os.Getenv("CIRCLE_SHA1")

	err := framework.HelmCmd(fmt.Sprintf("registry install --wait quay.io/giantswarm/kubernetes-external-dns-chart:%s -n test-deploy", channel))
	if err != nil {
		t.Errorf("unexpected error during installation of the chart: %v", err)
	}
	defer framework.HelmCmd("delete test-deploy --purge")

	err = checkDeployment()
	if err != nil {
		t.Fatalf("deployment manifest is incorrect: %v", err)
	}

	err = framework.HelmCmd("test --debug --cleanup test-deploy")
	if err != nil {
		t.Errorf("unexpected error during test of the chart: %v", err)
	}
}

// checkDeployment ensures that key properties of the external-dns deployment are
// correct.
func checkDeployment() error {
	name := "external-dns"
	expectedLabels := map[string]string{
		"app": "external-dns",
		"giantswarm.io/service-type": "managed",
	}
	expectedMatchLabels := map[string]string{
		"app": "external-dns",
	}

	c := f.K8sClient()
	d, err := c.Apps().Deployments(resourceNamespace).Get(name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return microerror.Newf("could not find deployment: '%s' %v", name, err)
	} else if err != nil {
		return microerror.Newf("unexpected error getting deployment: %v", err)
	}

	// Check deployment labels.
	if !reflect.DeepEqual(expectedLabels, d.ObjectMeta.Labels) {
		return microerror.Newf("expected labels: %v got: %v", expectedLabels, d.ObjectMeta.Labels)
	}

	// Check selector match labels.
	if !reflect.DeepEqual(expectedMatchLabels, d.Spec.Selector.MatchLabels) {
		return microerror.Newf("expected match labels: %v got: %v", expectedMatchLabels, d.Spec.Selector.MatchLabels)
	}

	// Check pod labels.
	if !reflect.DeepEqual(expectedLabels, d.Spec.Template.ObjectMeta.Labels) {
		return microerror.Newf("expected pod labels: %v got: %v", expectedLabels, d.Spec.Template.ObjectMeta.Labels)
	}

	return nil
}

package fixtures

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"

	rov1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/pkg/kubectl-argo-rollouts/cmd/abort"
	"github.com/argoproj/argo-rollouts/pkg/kubectl-argo-rollouts/cmd/promote"
	"github.com/argoproj/argo-rollouts/pkg/kubectl-argo-rollouts/cmd/restart"
	"github.com/argoproj/argo-rollouts/pkg/kubectl-argo-rollouts/cmd/retry"
	"github.com/argoproj/argo-rollouts/pkg/kubectl-argo-rollouts/cmd/set"
	"github.com/argoproj/argo-rollouts/pkg/kubectl-argo-rollouts/info"
)

type When struct {
	Common
	rollout *rov1.Rollout
}

func (w *When) ApplyManifests() *When {
	if w.rollout == nil {
		w.t.Fatal("No rollout to create")
	}
	var objects []runtime.Object
	for _, obj := range w.objects {
		objects = append(objects, obj)
	}
	objects = append(objects, w.rollout)

	for _, obj := range objects {
		objBytes, err := json.Marshal(obj)
		w.CheckError(err)
		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Env = os.Environ()
		cmd.Stdin = bytes.NewReader(objBytes)
		out, err := cmd.CombinedOutput()
		if err != nil {
			gvk := obj.GetObjectKind().GroupVersionKind()
			objMap, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
			un := unstructured.Unstructured{Object: objMap}
			w.log.Errorf("kubectl apply of %s %s failed: %s", gvk.Kind, un.GetName(), out)
			w.t.FailNow()
		}
		w.log.Info(string(out))
	}
	return w
}

func (w *When) UpdateImage(image string) *When {
	if w.rollout == nil {
		w.t.Fatal("Rollout not set")
	}
	err := set.SetImage(w.dynamicClient, w.namespace, w.rollout.Name, "*", image)
	w.CheckError(err)
	w.log.Infof("Updated image to %s", image)
	return w
}

func (w *When) PromoteRollout() *When {
	if w.rollout == nil {
		w.t.Fatal("Rollout not set")
	}
	_, err := promote.PromoteRollout(w.rolloutClient.ArgoprojV1alpha1().Rollouts(w.namespace), w.rollout.Name, false, false)
	w.CheckError(err)
	w.log.Info("Promoted rollout")
	return w
}

func (w *When) AbortRollout() *When {
	if w.rollout == nil {
		w.t.Fatal("Rollout not set")
	}
	_, err := abort.AbortRollout(w.rolloutClient.ArgoprojV1alpha1().Rollouts(w.namespace), w.rollout.Name)
	w.CheckError(err)
	w.log.Info("Aborted rollout")
	return w
}

func (w *When) RetryRollout() *When {
	if w.rollout == nil {
		w.t.Fatal("Rollout not set")
	}
	_, err := retry.RetryRollout(w.rolloutClient.ArgoprojV1alpha1().Rollouts(w.namespace), w.rollout.Name)
	w.CheckError(err)
	w.log.Info("Retried rollout")
	return w
}

func (w *When) RestartRollout() *When {
	if w.rollout == nil {
		w.t.Fatal("Rollout not set")
	}
	_, err := restart.RestartRollout(w.rolloutClient.ArgoprojV1alpha1().Rollouts(w.namespace), w.rollout.Name, nil)
	w.CheckError(err)
	w.log.Info("Restarted rollout")
	return w
}

func (w *When) WaitForRolloutStatus(status string) *When {
	checkStatus := func(ro *rov1.Rollout) bool {
		if info.RolloutStatusString(ro) == status {
			return true
		}
		return false
	}
	return w.WaitForRolloutCondition(checkStatus, fmt.Sprintf("status=%s", status), DefaultTimeout)
}

func (w *When) WaitForRolloutCanaryStepIndex(index int32) *When {
	checkStatus := func(ro *rov1.Rollout) bool {
		if ro.Status.CurrentStepIndex == nil {
			return false
		}
		return *ro.Status.CurrentStepIndex == index
	}
	return w.WaitForRolloutCondition(checkStatus, fmt.Sprintf("status.currentStepIndex=%d", index), DefaultTimeout)
}

func (w *When) WaitForRolloutCondition(test func(ro *rov1.Rollout) bool, condition string, timeout time.Duration) *When {
	start := time.Now()
	w.log.Infof("Waiting for condition: %s", condition)
	opts := metav1.ListOptions{FieldSelector: fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", w.rollout.Name)).String()}
	watch, err := w.rolloutClient.ArgoprojV1alpha1().Rollouts(w.namespace).Watch(opts)
	w.CheckError(err)
	defer watch.Stop()
	timeoutCh := make(chan bool, 1)
	go func() {
		time.Sleep(timeout)
		timeoutCh <- true
	}()
	for {
		select {
		case event := <-watch.ResultChan():
			wf, ok := event.Object.(*rov1.Rollout)
			if ok {
				if test(wf) {
					w.log.Infof("Condition '%s' met after %v", condition, time.Since(start).Truncate(time.Second))
					return w
				}
			} else {
				w.t.Fatal("not ok")
			}
		case <-timeoutCh:
			w.t.Fatalf("timeout after %v waiting for condition %s", timeout, condition)
		}
	}
}

func (w *When) DeleteRollout() *When {
	w.log.Info("Deleting")
	err := w.rolloutClient.ArgoprojV1alpha1().Rollouts(w.namespace).Delete(w.rollout.Name, nil)
	w.CheckError(err)
	return w
}

func (w *When) Then() *Then {
	return &Then{
		Common:  w.Common,
		rollout: w.rollout,
	}
}

func (w *When) Given() *Given {
	return &Given{
		Common:  w.Common,
		rollout: w.rollout,
	}
}

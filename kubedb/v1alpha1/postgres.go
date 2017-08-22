package v1alpha1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/appscode/kutil"
	"github.com/golang/glog"
	aci "github.com/k8sdb/apimachinery/api"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	"github.com/appscode/jsonpatch"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func CreateOrPatchPostgres(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(alert *aci.Postgres) *aci.Postgres) (*aci.Postgres, error) {
	cur, err := c.Postgreses(meta.Namespace).Get(meta.Name)
	if kerr.IsNotFound(err) {
		return c.Postgreses(meta.Namespace).Create(transform(&aci.Postgres{ObjectMeta: meta}))
	} else if err != nil {
		return nil, err
	}
	return PatchPostgres(c, cur, transform)
}

func PatchPostgres(c tcs.ExtensionInterface, cur *aci.Postgres, transform func(*aci.Postgres) *aci.Postgres) (*aci.Postgres, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	modJson, err := json.Marshal(transform(cur))
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreatePatch(curJson, modJson)
	if err != nil {
		return nil, err
	}
	if len(patch) == 0 {
		return cur, nil
	}
	pb, err := json.MarshalIndent(patch, "", "  ")
	if err != nil {
		return nil, err
	}
	glog.V(5).Infof("Patching Postgres %s@%s with %s.", cur.Name, cur.Namespace, string(pb))
	result, err := c.Postgreses(cur.Namespace).Patch(cur.Name, types.JSONPatchType, pb)
	return result, err
}

func TryPatchPostgres(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(*aci.Postgres) *aci.Postgres) (*aci.Postgres, error) {
	attempt := 0
	for ; attempt < kutil.MaxAttempts; attempt = attempt + 1 {
		cur, err := c.Postgreses(meta.Namespace).Get(meta.Name)
		if kerr.IsNotFound(err) {
			return cur, err
		} else if err == nil {
			return PatchPostgres(c, cur, transform)
		}
		glog.Errorf("Attempt %d failed to patch Postgres %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(kutil.RetryInterval)
	}
	return nil, fmt.Errorf("Failed to patch Postgres %s@%s after %d attempts.", meta.Name, meta.Namespace, attempt)
}

func TryUpdatePostgres(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(*aci.Postgres) *aci.Postgres) (*aci.Postgres, error) {
	attempt := 0
	for ; attempt < kutil.MaxAttempts; attempt = attempt + 1 {
		cur, err := c.Postgreses(meta.Namespace).Get(meta.Name)
		if kerr.IsNotFound(err) {
			return cur, err
		} else if err == nil {
			oJson, err := json.Marshal(cur)
			if err != nil {
				return nil, err
			}
			modified := transform(cur)
			mJson, err := json.Marshal(modified)
			if err != nil {
				return nil, err
			}
			if bytes.Equal(oJson, mJson) {
				return cur, err
			}

			result, err := c.Postgreses(cur.Namespace).Update(transform(cur))
			return result, err
		}
		glog.Errorf("Attempt %d failed to update Postgres %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(kutil.RetryInterval)
	}
	return nil, fmt.Errorf("Failed to update Postgres %s@%s after %d attempts.", meta.Name, meta.Namespace, attempt)
}

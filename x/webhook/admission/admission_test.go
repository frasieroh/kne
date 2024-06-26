// Copyright 2024 Google LLC
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

package admission

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	basicPod = `
	{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "r0",
        "namespace": "b2b"
    }
	}
	`
)

type fakeMutator struct {
	ret []byte
}

func (f *fakeMutator) MutateObject(_ runtime.Object) ([]byte, error) {
	return f.ret, nil
}

func TestPatchReviewResponse(t *testing.T) {
	uid := types.UID("test")
	patchType := admissionv1.PatchTypeJSONPatch
	patch := []byte(`not quite a real patch`)

	want := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:       uid,
			Allowed:   true,
			PatchType: &patchType,
			Patch:     patch,
		},
	}

	got, err := patchReviewResponse(uid, patch)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("patchReviewResponse(%q, %q) returned diff (-got, +want):\n%s", uid, patch, diff)
	}
}

func TestReviewResponse(t *testing.T) {
	uid := types.UID("test")
	reason := "fail!"

	want := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:     uid,
			Allowed: false,
			Result: &metav1.Status{
				Code:    418,
				Message: reason,
			},
		},
	}

	got := reviewResponse(uid, false, http.StatusTeapot, reason)
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("reviewResponse(%q, false, ...) returned diff (-got, +want):\n%s", uid, diff)
	}
}

func TestReview(t *testing.T) {
	patchType := admissionv1.PatchTypeJSONPatch
	tests := []struct {
		name     string
		inReq    *admissionv1.AdmissionRequest
		wantResp *admissionv1.AdmissionReview
		inMutor  mutator
		wantErr  bool
	}{
		{
			name: "webhook label",
			inReq: &admissionv1.AdmissionRequest{
				UID: types.UID("mutated"),
				Kind: metav1.GroupVersionKind{
					Kind: "Pod",
				},
				Object: runtime.RawExtension{
					Raw: []byte(basicPod),
				},
			},
			wantResp: &admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{
					Kind:       "AdmissionReview",
					APIVersion: "admission.k8s.io/v1",
				},
				Response: &admissionv1.AdmissionResponse{
					UID:       types.UID("mutated"),
					Allowed:   true,
					PatchType: &patchType,
					Patch:     []byte("notquiteapatch"),
				},
			},
			inMutor: &fakeMutator{
				ret: []byte("notquiteapatch"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Admitter{
				request: tt.inReq,
				mutator: tt.inMutor,
			}

			got, err := a.Review()
			if tt.wantErr != (err != nil) {
				t.Fatalf("Review() returned unexpected error. want: %t, got %t, error: %v", tt.wantErr, err != nil, err)
			}

			if diff := cmp.Diff(got, tt.wantResp); diff != "" {
				t.Errorf("Review() returned diff (-got, +want):\n%s", diff)
			}
		})
	}
}

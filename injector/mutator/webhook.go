package mutator

import (
	"bytes"
	"context"
	"github.com/linkerd/linkerd2/controller/k8s"
	"github.com/linkerd/linkerd2/controller/webhook"
	"html/template"
	"k8s.io/client-go/tools/record"

	admission "k8s.io/api/admission/v1beta1"
)

type Params struct {
}

func Mutate() webhook.Handler {
	return func(
		ctx context.Context,
		api *k8s.API,
		request *admission.AdmissionRequest,
		recorder record.EventRecorder,
	) (*admission.AdmissionResponse, error) {
		admissionResponse := &admission.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}

		params := Params{}

		t, err := template.New("patch").Parse(patch)
		if err != nil {
			return nil, err
		}

		var patchJSON bytes.Buffer
		if err = t.Execute(&patchJSON, params); err != nil {
			return nil, err
		}

		patchType := admission.PatchTypeJSONPatch
		admissionResponse.Patch = patchJSON.Bytes()
		admissionResponse.PatchType = &patchType

		return admissionResponse, nil
	}
}

/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"strings"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
	"knative.dev/serving/pkg/apis/autoscaling"
	apisconfig "knative.dev/serving/pkg/apis/config"
	"knative.dev/serving/pkg/apis/serving"
)

// Validate ensures Revision is properly configured.
func (r *Revision) Validate(ctx context.Context) *apis.FieldError {
	errs := serving.ValidateObjectMetadata(ctx, r.GetObjectMeta()).Also(
		r.ValidateLabels().ViaField("labels")).ViaField("metadata")
	errs = errs.Also(r.Status.Validate(apis.WithinStatus(ctx)).ViaField("status"))

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Revision)
		if diff, err := kmp.ShortDiff(original.Spec, r.Spec); err != nil {
			return &apis.FieldError{
				Message: "Failed to diff Revision",
				Paths:   []string{"spec"},
				Details: err.Error(),
			}
		} else if diff != "" {
			return &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"spec"},
				Details: diff,
			}
		}
	} else {
		errs = errs.Also(r.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
	}

	return errs
}

// Validate implements apis.Validatable
func (rts *RevisionTemplateSpec) Validate(ctx context.Context) *apis.FieldError {
	errs := rts.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec")
	errs = errs.Also(autoscaling.ValidateAnnotations(ctx, apisconfig.FromContextOrDefaults(ctx).Autoscaler,
		rts.GetAnnotations()).ViaField("metadata.annotations"))

	// If the RevisionTemplateSpec has a name specified, then check that
	// it follows the requirements on the name.
	errs = errs.Also(serving.ValidateRevisionName(ctx, rts.Name, rts.GenerateName))
	errs = errs.Also(serving.ValidateQueueSidecarAnnotation(rts.Annotations).ViaField("metadata.annotations"))
	return errs
}

// VerifyNameChange checks that if a user brought their own name previously that it
// changes at the appropriate times.
func (rts *RevisionTemplateSpec) VerifyNameChange(_ context.Context, og *RevisionTemplateSpec) *apis.FieldError {
	if rts.Name == "" {
		// We only check that Name changes when the RevisionTemplate changes.
		return nil
	}
	if rts.Name != og.Name {
		// The name changed, so we're good.
		return nil
	}

	diff, err := kmp.ShortDiff(og, rts)
	if err != nil {
		return &apis.FieldError{
			Message: "Failed to diff RevisionTemplate",
			Paths:   []string{apis.CurrentField},
			Details: err.Error(),
		}
	}
	if diff != "" {
		return &apis.FieldError{
			Message: "Saw the following changes without a name change (-old +new)",
			Paths:   []string{"metadata.name"},
			Details: diff,
		}
	}
	return nil
}

// Validate implements apis.Validatable
func (rs *RevisionSpec) Validate(ctx context.Context) *apis.FieldError {
	errs := serving.ValidatePodSpec(ctx, rs.PodSpec)

	if rs.TimeoutSeconds != nil {
		errs = errs.Also(serving.ValidateTimeoutSeconds(ctx, *rs.TimeoutSeconds))
	}

	if rs.ContainerConcurrency != nil {
		errs = errs.Also(serving.ValidateContainerConcurrency(ctx, rs.ContainerConcurrency).ViaField("containerConcurrency"))
	}

	return errs
}

// Validate implements apis.Validatable
func (rs *RevisionStatus) Validate(_ context.Context) *apis.FieldError {
	return nil
}

// ValidateLabels function validates service labels
func (r *Revision) ValidateLabels() (errs *apis.FieldError) {
	for key, val := range r.GetLabels() {
		switch key {
		case serving.RoutingStateLabelKey,
			serving.RouteLabelKey,
			serving.ServiceLabelKey,
			serving.ConfigurationGenerationLabelKey:
			// Known valid labels.
		case serving.ConfigurationLabelKey:
			errs = errs.Also(verifyLabelOwnerRef(val, serving.ConfigurationLabelKey, "Configuration", r.GetOwnerReferences()))
		default:
			if strings.HasPrefix(key, serving.GroupNamePrefix) {
				errs = errs.Also(apis.ErrInvalidKeyName(key, ""))
			}
		}
	}
	return
}

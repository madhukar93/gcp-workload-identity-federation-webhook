package webhooks

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("GCPWorkloadIdentityMutator.mutatePod", func() {
	var m *GCPWorkloadIdentityMutator
	var defaultMode int32 = 0400
	project := "demo"
	BeforeEach(func() {
		m = &GCPWorkloadIdentityMutator{
			AnnotationDomain:       annotationDomain,
			DefaultAudience:        AudienceDefault,
			DefaultTokenExpiration: DefaultTokenExpirationDefault,
			MinTokenExpration:      MinTokenExprationDefault,
			DefaultGCloudRegion:    DefaultGCloudRegionDefault,
			GcloudImage:            GcloudImageDefault,
			DefaultMode:            defaultMode,
			SetupContainerResources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
		}
	})
	When("passed Pod has unparsed token expiration annotation", func() {
		It("should raise error", func() {
			idConfig := GCPWorkloadIdentityConfig{
				WorkloadIdentityProvider: &workloadIdentityProviderFmt,
				ServiceAccountEmail:      fmt.Sprintf("sa@%s.iam.gserviceaccount.com", project),
				Audience:                 ptr.To("my-audience"),
				TokenExpirationSeconds:   ptr.To[int64](10000),
				RunAsUser:                ptr.To[int64](1000),
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						tokenExpirationAnnotation: "not-integer",
					},
				},
			}

			err := m.mutatePod(pod, idConfig)
			Expect(err).To(MatchError(ContainSubstring("must be positive integer string")))
		})
	})
	When("passed Pod does have conflicted and override fields", func() {
		It("should replace reqiured fields and override configurations", func() {
			idConfig := GCPWorkloadIdentityConfig{
				WorkloadIdentityProvider: &workloadIdentityProviderFmt,
				ServiceAccountEmail:      fmt.Sprintf("sa@%s.iam.gserviceaccount.com", project),
				Audience:                 ptr.To("my-audience"),
				TokenExpirationSeconds:   ptr.To[int64](10000),
				RunAsUser:                ptr.To[int64](1000),
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						idProviderAnnotation: "to-be-replaced",
						saEmailAnnotation:    "to-be-replaced",
						// belows can override idConfig values
						audienceAnnotation:        "my-audience",
						tokenExpirationAnnotation: "3601",
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						// to be replaced
						Name: K8sSATokenVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}},
					InitContainers: []corev1.Container{{
						// to be replaced
						Name:  GCloudSetupInitContainerName,
						Image: "busybox",
					}, {
						Name:  "ctr",
						Image: "busybox",
						VolumeMounts: []corev1.VolumeMount{{
							Name:      K8sSATokenVolumeName,
							MountPath: "/to/be/replaced",
						}},
						Env: []corev1.EnvVar{{
							Name:  "GOOGLE_APPLICATION_CREDENTIALS",
							Value: "to-be-replaced",
						}, {
							Name:  "CLOUDSDK_COMPUTE_REGION",
							Value: "not-to-be-replaced",
						}},
					}},
					Containers: []corev1.Container{{
						Name:  "ctr",
						Image: "busybox",
						VolumeMounts: []corev1.VolumeMount{{
							Name:      K8sSATokenVolumeName,
							MountPath: "/to/be/replaced",
						}},
						Env: []corev1.EnvVar{{
							Name:  "GOOGLE_APPLICATION_CREDENTIALS",
							Value: "to-be-replaced",
						}, {
							Name:  "CLOUDSDK_COMPUTE_REGION",
							Value: "not-to-be-replaced",
						}},
					}},
				},
			}

			err := m.mutatePod(pod, idConfig)
			Expect(err).NotTo(HaveOccurred())

			expectedEnvVars := []corev1.EnvVar{
				{
					Name:  "GOOGLE_APPLICATION_CREDENTIALS",
					Value: filepath.Join(GCloudConfigMountPath, ExternalCredConfigFilename),
				},
			}
			expectedEnvVars = append(expectedEnvVars, cloudSDKComputeRegionEnvVar("not-to-be-replaced")...)
			expectedEnvVars = append(expectedEnvVars, corev1.EnvVar{
				Name:  "CLOUDSDK_CONFIG",
				Value: GCloudConfigMountPath,
			})
			expectedEnvVars = append(expectedEnvVars, projectEnvVar(project)...)

			expected := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						idProviderAnnotation:      workloadIdentityProviderFmt,
						saEmailAnnotation:         idConfig.ServiceAccountEmail,
						audienceAnnotation:        "my-audience",
						tokenExpirationAnnotation: "3601",
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						gcloudSetupContainer(
							*idConfig.WorkloadIdentityProvider,
							idConfig.ServiceAccountEmail,
							project,
							m.GcloudImage,
							idConfig.RunAsUser,
							m.SetupContainerResources,
						), {
							Name:         "ctr",
							Image:        "busybox",
							VolumeMounts: volumeMountsToAddOrReplace(GCloudMode),
							Env:          expectedEnvVars,
						}},
					Containers: []corev1.Container{{
						Name:         "ctr",
						Image:        "busybox",
						VolumeMounts: volumeMountsToAddOrReplace(GCloudMode),
						Env:          expectedEnvVars,
					}},
					Volumes: m.volumesToAddOrReplace("my-audience", 3601, defaultMode, GCloudMode),
				},
			}
			// Expect(pod.Annotations).To(BeEquivalentTo(expected.Annotations))
			// Expect(pod.Spec.Volumes).To(BeEquivalentTo(expected.Spec.Volumes))
			// Expect(pod.Spec.InitContainers[0]).To(BeEquivalentTo(expected.Spec.InitContainers[0]))
			// Expect(pod.Spec.InitContainers[1]).To(BeEquivalentTo(expected.Spec.InitContainers[1]))
			// Expect(pod.Spec.Containers).To(BeEquivalentTo(expected.Spec.Containers))
			// Expect(pod).To(BeEquivalentTo(expected))
			Expect(cmp.Diff(pod, expected)).To(BeEmpty())
		})
	})
	When("passed Pod doesn't have no conflicted and no override fields", func() {
		It("should mutate required fields", func() {
			idConfig := GCPWorkloadIdentityConfig{
				WorkloadIdentityProvider: &workloadIdentityProviderFmt,
				ServiceAccountEmail:      fmt.Sprintf("sa@%s.iam.gserviceaccount.com", project),
			}
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Name:  "ctr",
						Image: "busybox",
					}},
					Containers: []corev1.Container{{
						Name:  "ctr",
						Image: "busybox",
					}},
				},
			}

			err := m.mutatePod(pod, idConfig)
			Expect(err).NotTo(HaveOccurred())

			expected := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						idProviderAnnotation:      workloadIdentityProviderFmt,
						saEmailAnnotation:         idConfig.ServiceAccountEmail,
						audienceAnnotation:        m.DefaultAudience,
						tokenExpirationAnnotation: fmt.Sprint(int64(m.DefaultTokenExpiration.Seconds())),
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						gcloudSetupContainer(
							*idConfig.WorkloadIdentityProvider,
							idConfig.ServiceAccountEmail,
							project,
							m.GcloudImage,
							idConfig.RunAsUser,
							m.SetupContainerResources,
						), {
							Name:         "ctr",
							Image:        "busybox",
							VolumeMounts: volumeMountsToAddOrReplace(GCloudMode),
							Env:          append(envVarsToAddOrReplace(idConfig.InjectionMode), envVarsToAddIfNotPresent(m.DefaultGCloudRegion, project)...),
						},
					},
					Containers: []corev1.Container{{
						Name:         "ctr",
						Image:        "busybox",
						VolumeMounts: volumeMountsToAddOrReplace(GCloudMode),
						Env:          append(envVarsToAddOrReplace(idConfig.InjectionMode), envVarsToAddIfNotPresent(m.DefaultGCloudRegion, project)...),
					}},
					Volumes: m.volumesToAddOrReplace(
						m.DefaultAudience,
						(int64)(m.DefaultTokenExpiration.Seconds()),
						defaultMode,
						GCloudMode,
					),
				},
			}
			// Expect(pod.Annotations).To(BeEquivalentTo(expected.Annotations))
			// Expect(pod.Spec.Volumes).To(BeEquivalentTo(expected.Spec.Volumes))
			// Expect(pod.Spec.InitContainers).To(BeEquivalentTo(expected.Spec.InitContainers))
			// Expect(pod.Spec.Containers).To(BeEquivalentTo(expected.Spec.Containers))
			// Expect(pod).To(BeEquivalentTo(expected))
			Expect(cmp.Diff(pod, expected)).To(BeEmpty())
		})
	})
})

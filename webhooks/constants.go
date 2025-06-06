package webhooks

import "time"

const (
	// Defaults
	AnnotationDomainDefault       = "cloud.google.com"
	AudienceDefault               = "sts.googleapis.com"
	DefaultTokenExpirationDefault = time.Duration(24) * time.Hour
	MinTokenExprationDefault      = time.Duration(1) * time.Hour
	DefaultGCloudRegionDefault    = "asia-northeast1"
	GcloudImageDefault            = "gcr.io/google.com/cloudsdktool/google-cloud-cli:stable"
	VolumeModeDefault             = 0440
	SetupContainerResources       = ""

	// Constants for injected fields
	DirectInjectedExternalVolumeName = "external-credential-config"
	DirectInjectedExternalMountPath  = "/var/run/secrets/workload-identity"
	ExternalCredConfigFilename       = "federation.json"
	K8sSATokenVolumeName             = "gcp-iam-token"
	K8sSATokenMountPath              = "/var/run/secrets/sts.googleapis.com/serviceaccount"
	K8sSATokenName                   = "token"
	GCloudConfigVolumeName           = "gcloud-config"
	GCloudConfigMountPath            = "/var/run/secrets/gcloud/config"
	GCloudSetupInitContainerName     = "gcloud-setup"
)

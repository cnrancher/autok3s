package common

import (
	"github.com/cnrancher/autok3s/pkg/types/alibaba"
	"github.com/cnrancher/autok3s/pkg/types/aws"
	"github.com/cnrancher/autok3s/pkg/types/google"
	"github.com/cnrancher/autok3s/pkg/types/tencent"
)

var DefaultTemplates = map[string]interface{}{
	"aws": aws.Options{
		AMI:                    "ami-007855ac798b5175e", // Canonical, Ubuntu, 22.04 LTS, amd64 jammy image build on 2023-03-25
		InstanceType:           "t3a.medium",            // 2c/4g
		VolumeType:             "gp3",
		RootSize:               "16",
		Region:                 "us-east-1",
		Zone:                   "us-east-1a",
		RequestSpotInstance:    false,
		CloudControllerManager: false,
	},
	"alibaba": alibaba.Options{
		Image:                   "ubuntu_22_04_x64_20G_alibase_20230208.vhd", // Ubuntu 22.04 64 bit
		InstanceType:            "ecs.c6.large",                              // 2c/4g
		InternetMaxBandwidthOut: "5",
		DiskCategory:            "cloud_essd",
		DiskSize:                "40",
		EIP:                     false,
		CloudControllerManager:  false,
		Region:                  "cn-hangzhou",
		Zone:                    "cn-hangzhou-i",
		SpotStrategy:            "NoSpot",
		SpotDuration:            1,
	},
	"tencent": tencent.Options{
		ImageID:                 "img-487zeit5", // Ubuntu 22.04 LTS 64 bit
		InstanceType:            "S5.MEDIUM4",   // 2c/4g
		InstanceChargeType:      "POSTPAID_BY_HOUR",
		InternetMaxBandwidthOut: "5",
		SystemDiskType:          "CLOUD_SSD",
		SystemDiskSize:          "50",
		Region:                  "ap-guangzhou",
		Zone:                    "ap-guangzhou-3",
		PublicIPAssignedEIP:     false,
		Spot:                    false,
		CloudControllerManager:  false,
	},
	"google": google.Options{
		Region:       "us-central1",
		Zone:         "us-central1-b",
		MachineType:  "e2-medium",
		MachineImage: "ubuntu-os-cloud/global/images/ubuntu-2204-jammy-v20230429", // Ubuntu 22.04 amd64 jammy image built on 2023-04-29
		DiskType:     "pd-balanced",
		DiskSize:     10,
		VMNetwork:    "default",
		Scopes:       "https://www.googleapis.com/auth/devstorage.read_only,https://www.googleapis.com/auth/logging.write,https://www.googleapis.com/auth/monitoring.write,https://www.googleapis.com/auth/cloud-platform",
	},
}

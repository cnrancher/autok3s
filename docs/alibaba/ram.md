# RAMs
Alibaba Cloud Provider RAMs:
```json
{
    "Statement": [
        {
            "Action": [
                "ecs:CreateNetworkInterface",
                "ecs:DescribeNetworkInterfaces",
                "ecs:AttachNetworkInterface",
                "ecs:DetachNetworkInterface",
                "ecs:DeleteNetworkInterface",
                "ecs:DescribeInstanceAttribute",
                "ecs:DescribeInstanceTypesNew",
                "ecs:AssignPrivateIpAddresses",
                "ecs:UnassignPrivateIpAddresses",
                "ecs:DescribeInstances",
                "ecs:DeleteInstances",
                "ecs:DescribeInstanceStatus",
                "ecs:RunInstances",
                "ecs:ListTagResources",
                "ecs:StartInstances",
                "ecs:StopInstances"
            ],
            "Resource": [
                "*"
            ],
            "Effect": "Allow"
        },
        {
            "Action": [
                "vpc:DescribeVSwitches",
                "vpc:CreateVSwitch",
                "vpc:DeleteVSwitch",
                "vpc:DescribeVSwitches",
                "vpc:DescribeVpcs",
                "vpc:TagResources",
                "vpc:AllocateEipAddress",
                "vpc:AssociateEipAddress",
                "vpc:DescribeEipAddresses",
                "vpc:UnassociateEipAddress",
                "vpc:ReleaseEipAddress",
                "vpc:ListTagResources"
            ],
            "Resource": [
                "*"
            ],
            "Effect": "Allow"
        }
    ],
    "Version": "1"
}
```

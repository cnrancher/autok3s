# RAMs
Alibaba Cloud Provider RAMs:
```json
{
  "Version": "1",
  "Statement": [
    {
      "Action": [
        "ecs:Describe*",
        "ecs:AttachDisk",
        "ecs:CreateDisk",
        "ecs:CreateSnapshot",
        "ecs:CreateRouteEntry",
        "ecs:DeleteDisk",
        "ecs:DeleteSnapshot",
        "ecs:DeleteRouteEntry",
        "ecs:DetachDisk",
        "ecs:ModifyAutoSnapshotPolicyEx",
        "ecs:ModifyDiskAttribute",
        "ecs:CreateNetworkInterface",
        "ecs:AttachNetworkInterface",
        "ecs:DetachNetworkInterface",
        "ecs:DeleteNetworkInterface",
        "ecs:CreateNetworkInterface",
        "ecs:AttachNetworkInterface",
        "ecs:DetachNetworkInterface",
        "ecs:DeleteNetworkInterface",
        "ecs:AssignPrivateIpAddresses",
        "ecs:UnassignPrivateIpAddresses",
        "ecs:DeleteInstances",
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
        "cr:Get*",
        "cr:List*",
        "cr:PullRepository"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "slb:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "cms:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "vpc:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "log:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "nas:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    }
  ]
}
```

# RAMs
Tencent Cloud Provider RAMs:
```json
{
    "version": "2.0",
    "statement": [
        {
            "action": [
                "cvm:RunInstances",
                "cvm:DescribeInstances",
                "cvm:TerminateInstances",
                "cvm:StartInstances",
                "cvm:StopInstances",
                "cvm:DescribeInstancesStatus",
                "cvm:AllocateAddresses",
                "cvm:ReleaseAddresses",
                "cvm:AssociateAddress",
                "cvm:DisassociateAddress",
                "cvm:DescribeAddresses",
                "cvm:DescribeImages"
            ],
            "resource": "*",
            "effect": "allow"
        },
        {
            "action": [
                "vpc:*"
            ],
            "resource": "*",
            "effect": "allow"
        },
        {
            "action": [
                "tag:AddResourceTag",
                "tag:DescribeResourcesByTags",
                "tag:AttachResourcesTag"
            ],
            "resource": "*",
            "effect": "allow"
        },
        {
            "action": [
                "ccs:Describe*",
                "ccs:CreateClusterRoute"
            ],
            "resource": "*",
            "effect": "allow"
        },
        {
            "action": [
                "clb:*"
            ],
            "resource": "*",
            "effect": "allow"
        }
    ]
}
```

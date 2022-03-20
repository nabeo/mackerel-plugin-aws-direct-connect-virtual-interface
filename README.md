# mackerel-plugin-aws-direct-connect-virtual-interface

```
$ mackerel-plugin-aws-direct-connect-virtual-interface -help
Usage of mackerel-plugin-aws-direct-connect-virtual-interface:
  -access-key-id string
        AWS Access Key ID
  -direct-connect-connection string
        Resource ID of Direct Connect
  -metric-key-prefix string
        Metric Key Prefix
  -region string
        AWS Region
  -role-arn string
        IAM Role ARN for assume role
  -secret-key-id string
        AWS Secret Access Key ID
  -virtual-interface-id string
        Resource ID of Direct Connect Virtual Interface
$
```

## use Assume Role

create IAM Role with the AWS Account that created Transit Gateway Attachment.

- no MFA
- allowed Policy
    - CloudWatchReadOnlyAccess

create IAM Policy with the AWS Account that runs mackerel-plugin-aws-transitgateway-attachment.

```json
{
    "Version": "2012-10-17",
    "Statement": {
        "Effect": "Allow",
        "Action": "sts:AssumeRole",
        "Resource": "arn:aws:iam::123456789012:role/YourIAMRoleName"
    }
}
```

attach IAM Policy to AWS Resouce that runs mackerel-plugin-aws-transitgateway-attachment.

## Synopsis

use assume role.
```shell
mackerel-plugin-aws-direct-connect-virtual-interface -role-arn <IAM Role Arn> -region <region> \
                                                     -direct-connect-connection <Resource ID of Direct Connect> \
                                                     -virtual-interface-id <Resource ID of Direct Connect Virtual Interface>
```

use access key id and secret key.
```shell
mackerel-plugin-aws-direct-connect-virtual-interface -region <region> \
                                                     -direct-connect-connection <Resource ID of Direct Connect> \
                                                     -virtual-interface-id <Resource ID of Direct Connect Virtual Interface>
                                                    [-access-key-id <AWS Access Key ID> -secret-key-id <WS Secret Access Key ID>] \
```

# ilert-kube-agent-serverless

## Requirements

- Kubernetes cluster 1.15+
- AWS account with IAM, Lambda and S3 permissions
- iLert alert source API key

## Deployment

1. Add ilert-kube-agent role to the aws-auth config map:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth
  namespace: kube-system
data:
  mapRoles: |
    # ...
    # Your operator roles map here
    # ...
    - roleARN: arn:aws:iam::000000000000:role/ilert-kube-agent
      username: ilert-kube-agent
      groups:
      - system:masters
    # ...
```

> NOTE: Please change the AWS account id before apply the config. In this example `000000000000` is the aws account id.

If you have used the `terraform-aws-modules/eks/aws` module to create your kubernetes cluster, you can define this config as follow:

```tf

module "my-cluster" {
  source          = "terraform-aws-modules/eks/aws"
  cluster_name    = "my-cluster"
  cluster_version = "1.17"

  // your cluster configuration is here
  // ...

  map_roles = [
    // ...
    // Your operator roles map here
    // ...
    {
      rolearn  = "arn:aws:iam::000000000000:role/ilert-kube-agent"
      username = "ilert-kube-agent"
      groups   = ["system:masters"]
    },
  ]
}
```

2. Build the ilert-kube-agent-serverless binary:

```sh
cd ./deployment/lambda
make build
```

3. Install serverless dependencies

```sh
npm install
```

4. Deploy lambda function

```sh
serverless deploy --conceal --verbose --cluster=<YOUR KUBERNETES CLUSTER NAME HERE> --region=<YOUR KUBERNETES CLUSTER REGION HERE> --api-key=<YOUR KEY HERE>
```

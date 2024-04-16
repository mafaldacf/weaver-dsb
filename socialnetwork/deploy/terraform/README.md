# Terraform for Weaver-DeathStarBench-SocialNetwork Benchmark

Automation tool code to build and deploy AWS resources used in the benchmark.


## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)
- [GCP Cli](todo)

## Getting Started

Configure the AWS profile settings:

``` zsh
    aws configure
```

Initialize the working directory:

``` zsh
    gsutil config -a
    terraform init
    terraform apply
    terraform destroy
```

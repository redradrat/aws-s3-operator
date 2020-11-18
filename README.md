# AWS S3 Operator

Super early stages, but yeah... does create, update and delete an S3 bucket.

Best used with the [AWS IAM Operator](https://github.com/redradrat/aws-iam-operator). 
Which will allow you to create users etc.

## How to run

You have to pass `--region` to the operator. For example running the docker image
with a region flag. e.g. `--region eu-central-1`

## Example

Please see [examples](./examples) for an example manifest.
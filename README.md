# codepipeline-modify-src

Allows you to modify artifacts in a code pipeline build.

# Installing

`S3_BUCKET=<bucket name> make install`

# Usage

As a stage in a Codepipeline:

```yaml
        - Name: Build
          Actions:
            - Name: ModifySource
              ActionTypeId:
                Category: Invoke
                Owner: AWS
                Provider: Lambda
                Version: 1
              Configuration:
                FunctionName: !ImportValue 'ModifySourceLambdaFnName'
                UserParameters: 'mybucket/files.zip'
              InputArtifacts:
                - Name: Source
              OutputArtifacts:
                - Name: ModifiedSource
              RunOrder: 1
```
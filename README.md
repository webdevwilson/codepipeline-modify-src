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
                FunctionName: ModifySourceArtifactLambda
                UserParameters: 'mybucket/files.zip'
              InputArtifacts:
                - Name: Source
              OutputArtifacts:
                - Name: ModifiedSource
              RunOrder: 1
```

Your pipeline will also need the permissions to invoke the function:

```yaml
              - Effect: 'Allow'
                Action:
                  - 's3:GetObject'
                  - 's3:GetObjectVersion'
                Resource:
                  - !Sub 'arn:aws:s3:::<BUCKET_NAME>'
                  - !Sub 'arn:aws:s3:::<BUCKET_NAME>/*'
              - Effect: 'Allow'
                Action:
                  - 'lambda:InvokeFunction'
                Resource:
                  - 'arn:aws:lambda:${AWS::Region}:${AWS::AccountId}:function:ModifySourceArtifactLambda'
```
# codepipeline-modify-src

Allows you to modify artifacts in a CodePipeline build. This enables you to share
common files between projects (like buildspecs, for example).

# Installing

`S3_BUCKET=<bucket name> make install`

# Usage

Call the pipeline as a stage in a CodePipeline, passing in the bucket and key of a zip file.
The zip file will be extracted inside the input artifact and output.

In CloudFormation:

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
                UserParameters: '<bucketname>/<myfile>.zip'
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
package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

var s = session.Must(session.NewSession())
var codepipelineSvc = codepipeline.New(s)
var s3Svc = s3.New(s)

func handler(event events.CodePipelineEvent) {

	// recover from panics
	defer func() {
		if r := recover(); r != nil {
			var err = fmt.Errorf("panic: %v", r)
			var msg = fmt.Sprintf("%v", err)
			log.Printf("panic: %v", r)
			jobFailure(&event.CodePipelineJob.ID, &msg)
		}
	}()

	// print out event
	bytes, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}
	log.Printf("Received event: %s", string(bytes))


	var pipelineS3Svc = createS3Svc(event.CodePipelineJob.Data.ArtifactCredentials)

	// create a zip file
	tmpFile, err := ioutil.TempFile("", "*")
	if err != nil {
		panic(err)
	}

	dest := zip.NewWriter(tmpFile)

	// pull the artifact and add to zip file
	var artifact = event.CodePipelineJob.Data.InputArtifacts[0].Location.S3Location
	log.Printf("Pulling source artifact from S3: %s %s", artifact.BucketName, artifact.ObjectKey)
	addS3FilesToZip(pipelineS3Svc, &artifact.BucketName, &artifact.ObjectKey, dest)

	// read the zip overlay from the user parameters and write to a temporary file
	var bucket, key = getBucketAndKey(event.CodePipelineJob.Data.ActionConfiguration.Configuration.UserParameters)
	log.Printf("Pulling overlay from S3: %s %s", bucket, key)

	addS3FilesToZip(s3Svc, &bucket, &key, dest)

	log.Printf("Writing zip file")

	err = dest.Close()
	if err != nil {
		panic(err)
	}

	err = tmpFile.Close()
	if err != nil {
		panic(err)
	}

	tmpFile, err = os.Open(tmpFile.Name())
	if err != nil {
		panic(err)
	}

	// upload output artifacts
	var outputS3 = event.CodePipelineJob.Data.OutPutArtifacts[0].Location.S3Location
	log.Printf("Uploading output artifacts to S3: %s %s", outputS3.ObjectKey, outputS3.BucketName)
	pipelineS3Svc.PutObject(&s3.PutObjectInput{
		Body: tmpFile,
		Key: &outputS3.ObjectKey,
		Bucket: &outputS3.BucketName,
	})

	// upload job success
	log.Printf("Job successful")
	codepipelineSvc.PutJobSuccessResult(&codepipeline.PutJobSuccessResultInput{
		JobId: &event.CodePipelineJob.ID,
	})
}

func createS3Svc(creds events.CodePipelineArtifactCredentials) *s3.S3 {
	var pipelineSession = session.Must(session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken),
	}))
	return s3.New(pipelineSession)
}

func addS3FilesToZip(svc *s3.S3, bucket *string, key *string, dest *zip.Writer) {
	var f = readZipFileFromS3(svc, bucket, key)
	var src, err = zip.OpenReader(f)

	if err != nil {
		panic(err)
	}

	addFilesToZip(src, dest)
}

func readZipFileFromS3(svc *s3.S3, bucket *string, key *string) string {
	resp, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: bucket,
		Key: key,
	})

	if err != nil {
		panic(err)
	}

	file, err := ioutil.TempFile("", "*")
	defer file.Close()

	if err != nil {
		panic(err)
	}

	_, err = io.Copy(file, resp.Body)

	if err != nil {
		panic(err)
	}

	return file.Name()
}

func addFilesToZip(src *zip.ReadCloser, dest *zip.Writer) {
	for _, f := range src.File {
		addFileToZip(f, dest)
	}
}

func addFileToZip(src *zip.File, dest *zip.Writer) {

	// create the zip header
	header, err := zip.FileInfoHeader(src.FileInfo())
	if err != nil {
		panic(err)
	}
	header.Name = src.Name
	header.Method = zip.Deflate

	// create the header
	writer, err := dest.CreateHeader(header)
	if err != nil {
		panic(err)
	}

	// open the reader for the source
	srcReader, err := src.Open()
	defer srcReader.Close()
	if err != nil {
		panic(err)
	}

	// copy file into zip
	_, err = io.Copy(writer, srcReader)
	if err != nil {
		panic(err)
	}
}

// return the bucket, and key from the string
func getBucketAndKey(path string) (string, string) {
	parts := strings.Split(path, "/")
	return parts[0], strings.Join(parts[1:], "/")
}

func jobFailure(jobId *string, msg *string) {
	codepipelineSvc.PutJobFailureResult(&codepipeline.PutJobFailureResultInput{
		FailureDetails: &codepipeline.FailureDetails{
			Message:             msg,
			Type:                nil,
		},
		JobId: jobId,
	})
}

func main() {
	lambda.Start(handler)
}
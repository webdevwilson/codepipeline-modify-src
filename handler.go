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

	// attempt to gracefully recovery from panics
	defer func() {
		if r := recover(); r != nil {
			var err = fmt.Errorf("panic: %v", r)
			log.Printf("panic: %v", r)
			jobFailure(&event.CodePipelineJob.ID, err)
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
	err = addS3FilesToZip(pipelineS3Svc, &artifact.BucketName, &artifact.ObjectKey, dest)
	if err != nil {
		jobFailure(&event.CodePipelineJob.ID, err)
		return
	}

	// read the zip overlay from the user parameters and write to a temporary file
	var bucket, key = getBucketAndKey(event.CodePipelineJob.Data.ActionConfiguration.Configuration.UserParameters)
	log.Printf("Pulling overlay from S3: %s %s", bucket, key)

	err = addS3FilesToZip(s3Svc, &bucket, &key, dest)
	if err != nil {
		jobFailure(&event.CodePipelineJob.ID, err)
		return
	}

	log.Printf("Writing zip file")

	err = dest.Close()
	if err != nil {
		jobFailure(&event.CodePipelineJob.ID, err)
		return
	}

	err = tmpFile.Close()
	if err != nil {
		jobFailure(&event.CodePipelineJob.ID, err)
		return
	}

	tmpFile, err = os.Open(tmpFile.Name())
	if err != nil {
		jobFailure(&event.CodePipelineJob.ID, err)
		return
	}

	// upload output artifacts
	var outputS3 = event.CodePipelineJob.Data.OutPutArtifacts[0].Location.S3Location
	log.Printf("Uploading output artifacts to S3: %s %s", outputS3.ObjectKey, outputS3.BucketName)
	_, err = pipelineS3Svc.PutObject(&s3.PutObjectInput{
		Body: tmpFile,
		Key: &outputS3.ObjectKey,
		Bucket: &outputS3.BucketName,
	})

	if err != nil {
		jobFailure(&event.CodePipelineJob.ID, err)
		return
	}

	// upload job success
	log.Printf("Job successful")
	_, err = codepipelineSvc.PutJobSuccessResult(&codepipeline.PutJobSuccessResultInput{
		JobId: &event.CodePipelineJob.ID,
	})

	if err != nil {
		log.Printf("Error reporting job success: %s", err)
	}
}

func createS3Svc(creds events.CodePipelineArtifactCredentials) *s3.S3 {
	var pipelineSession = session.Must(session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken),
	}))
	return s3.New(pipelineSession)
}

func addS3FilesToZip(svc *s3.S3, bucket *string, key *string, dest *zip.Writer) error {
	var f, err = readZipFileFromS3(svc, bucket, key)

	if err != nil {
		return err
	}

	var src *zip.ReadCloser
	src, err = zip.OpenReader(f)

	if err != nil {
		return err
	}

	err = addFilesToZip(src, dest)
	if err != nil {
		return err
	}

	return nil
}

func readZipFileFromS3(svc *s3.S3, bucket *string, key *string) (string, error) {
	resp, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: bucket,
		Key: key,
	})

	if err != nil {
		return "", err
	}

	file, err := ioutil.TempFile("", "*")

	if err != nil {
		return "", err
	}

	_, err = io.Copy(file, resp.Body)

	if err != nil {
		return "", err
	}

	return file.Name(), file.Close()
}

func addFilesToZip(src *zip.ReadCloser, dest *zip.Writer) error {
	for _, f := range src.File {
		err := addFileToZip(f, dest)
		if err != nil {
			return err
		}
	}

	return nil
}

func addFileToZip(src *zip.File, dest *zip.Writer) error {

	// create the zip header
	header, err := zip.FileInfoHeader(src.FileInfo())
	if err != nil {
		return err
	}

	header.Name = src.Name
	header.Method = zip.Deflate

	// create the header
	writer, err := dest.CreateHeader(header)
	if err != nil {
		return err
	}

	// open the reader for the source
	srcReader, err := src.Open()

	if err != nil {
		return err
	}

	// copy file into zip
	_, err = io.Copy(writer, srcReader)
	if err != nil {
		return err
	}

	return srcReader.Close()
}

// return the bucket, and key from the string
func getBucketAndKey(path string) (string, string) {
	parts := strings.Split(path, "/")
	return parts[0], strings.Join(parts[1:], "/")
}

func jobFailure(jobId *string, err error) {
	msg := fmt.Sprintf("Job Failure: %v", err)
	log.Println(msg)
	_, err = codepipelineSvc.PutJobFailureResult(&codepipeline.PutJobFailureResultInput{
		FailureDetails: &codepipeline.FailureDetails{
			Message:             &msg,
			Type:                nil,
		},
		JobId: jobId,
	})

	log.Printf("Error reporting job failure: %s", err)
}

func main() {
	lambda.Start(handler)
}
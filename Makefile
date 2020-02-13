INSTALL_PKG := install.pkg.yaml

clean:
	rm $(INSTALL_PKG)

handler:
	go build handler

$(INSTALL_PKG):
	aws cloudformation package \
		--template-file install.yaml \
		--s3-bucket $(S3_BUCKET) \
		--output-template-file $(INSTALL_PKG)

install: $(INSTALL_PKG)
	aws cloudformation deploy \
		--template-file $(INSTALL_PKG) \
		--stack-name codepipeline-modify-src \
		--capabilities CAPABILITY_IAM \
		--region $(AWS_REGION)

.PHONY := clean install
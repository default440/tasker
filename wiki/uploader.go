package wiki

import (
	"strconv"

	"github.com/google/uuid"
)

func UploadContent(api *API, pageID, content, contentType string, opts ...UploadOption) error {
	if IsMarkdownContentType(contentType) {
		content = `` +
			`<ac:structured-macro ac:name="markdown" ac:schema-version="1" ac:macro-id="` + uuid.NewString() + `">
				<ac:parameter ac:name="allowHtml">true</ac:parameter>
  				<ac:parameter ac:name="headerLinks">true</ac:parameter>
				<ac:parameter ac:name="atlassian-macro-output-type">BLOCK</ac:parameter>
				<ac:plain-text-body><![CDATA[` + content + `]]></ac:plain-text-body>
			</ac:structured-macro>`
		contentType = "storage"
	}

	options := getUploadOptions(opts...)

	if options.addTableOfContents {
		content = `<ac:structured-macro xmlns:ac="http://atlassian.com/content" ac:name="expand" ac:schema-version="1" ac:macro-id="` + uuid.NewString() + `">
			<ac:parameter ac:name="title">Table of Contents</ac:parameter>
			<ac:rich-text-body>
				<p>
					<ac:structured-macro ac:name="toc" ac:schema-version="1" ac:macro-id="` + uuid.NewString() + `">
						<ac:parameter ac:name="maxLevel">` + strconv.Itoa(options.headerLevel) + `</ac:parameter>
					</ac:structured-macro>
				</p>
			</ac:rich-text-body>
		</ac:structured-macro>
		` + content
	}

	return api.UploadContent(pageID, content, contentType)
}

func IsMarkdownContentType(contentType string) bool {
	return contentType == "md" || contentType == "markdown"
}

type UploadOptions struct {
	addTableOfContents bool
	headerLevel        int
}

type UploadOption func(options *UploadOptions)

func AddTableOfContents(add bool) UploadOption {
	return func(options *UploadOptions) { options.addTableOfContents = add }
}

func HeaderLevel(level int) UploadOption {
	return func(options *UploadOptions) { options.headerLevel = level }
}

func getUploadOptions(opts ...UploadOption) UploadOptions {
	options := UploadOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return options
}

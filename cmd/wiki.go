package cmd

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"tasker/wiki"

	"github.com/google/uuid"
	"github.com/pterm/pterm"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	goconfluence "github.com/virtomize/confluence-go-api"
)

var (
	wikiCmd = &cobra.Command{
		Use:   "wiki",
		Short: "Manage Wiki pages",
		Long:  `Move wike pages.`,
	}

	moveWikiCmd = &cobra.Command{
		Use:   "move <Page ID|Title, ...>",
		Short: "Move wiki pages",
		Long: `Replace wiki pages under new parent.
If page titles used, space key required.`,
		Args: cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			moveWikiCmdFlagMovingPages = append(moveWikiCmdFlagMovingPages, args...)

			if moveWikiCmdFlagPagesSpaceKey == "" {
				if _, err := strconv.Atoi(moveWikiCmdFlagNewParentPage); err != nil {
					cobra.CheckErr(errors.New("space key required when page titles used"))
					return
				}

				pageIDsCount := lo.CountBy(moveWikiCmdFlagMovingPages, func(page string) bool {
					_, err := strconv.Atoi(page)
					return err == nil
				})

				if pageIDsCount != len(moveWikiCmdFlagMovingPages) {
					cobra.CheckErr(errors.New("space key required when page titles used"))
					return
				}
			}

			err := moveWikiPagesCommand()
			cobra.CheckErr(err)
		},
	}

	uploadWikiContentCmd = &cobra.Command{
		Use:   "upload",
		Short: "Upload wiki page content",
		Long:  `Upload wiki page content markup.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := uploadWikiPageContentCommand()
			cobra.CheckErr(err)
		},
	}

	getWikiContentCmd = &cobra.Command{
		Use:   "get <PageID>",
		Short: "Get wiki page content",
		Long:  `Retrieve wiki page content markup.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := getWikiPageContentCommand(args[0])
			cobra.CheckErr(err)
		},
	}

	queryWikiPagesCmd = &cobra.Command{
		Use:   "query <query>",
		Short: "Query wiki pages",
		Long:  `Retrieve wiki pages by query.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := queryWikiPagesCommand(args[0])
			cobra.CheckErr(err)
		},
	}

	moveWikiCmdFlagNewParentPage string
	moveWikiCmdFlagMovingPages   []string
	moveWikiCmdFlagPagesSpaceKey string

	uploadWikiContentCmdFlagTargetID           uint
	uploadWikiContentCmdFlagSourcePath         string
	uploadWikiContentCmdFlagContentType        string
	uploadWikiContentCmdFlagAddTableOfContents bool
	uploadWikiContentCmdFlagHeaderLevel        uint
	uploadWikiContentCmdFlagFixRefs            bool

	queryWikiPagesCmdFlagShowContent bool
	queryWikiPagesCmdFlagShowId      bool
)

func init() {
	rootCmd.AddCommand(wikiCmd)
	wikiCmd.AddCommand(moveWikiCmd)
	wikiCmd.AddCommand(uploadWikiContentCmd)
	wikiCmd.AddCommand(getWikiContentCmd)
	wikiCmd.AddCommand(queryWikiPagesCmd)

	moveWikiCmd.Flags().StringVarP(&moveWikiCmdFlagNewParentPage, "target", "t", "", "ID or title of target parent Wiki page")
	moveWikiCmd.Flags().StringSliceVarP(&moveWikiCmdFlagMovingPages, "page", "p", nil, "ID or title of moving page")
	moveWikiCmd.Flags().StringVarP(&moveWikiCmdFlagPagesSpaceKey, "space-key", "s", "", "Space Key of pages")
	cobra.CheckErr(moveWikiCmd.MarkFlagRequired("target"))

	uploadWikiContentCmd.Flags().UintVarP(&uploadWikiContentCmdFlagTargetID, "target", "t", 0, "ID of target Wiki page")
	uploadWikiContentCmd.Flags().StringVarP(&uploadWikiContentCmdFlagSourcePath, "file", "f", "", "Path to file with wiki markup")
	uploadWikiContentCmd.Flags().StringVarP(&uploadWikiContentCmdFlagContentType, "type", "", "wiki", "Content type (wiki, storage, editor, md, etc.)")
	uploadWikiContentCmd.Flags().BoolVarP(&uploadWikiContentCmdFlagAddTableOfContents, "add-table-of-contents", "", false, "Perepend content with 'Table of Contents' wiki macros")
	uploadWikiContentCmd.Flags().UintVarP(&uploadWikiContentCmdFlagHeaderLevel, "header-level", "", 2, "Max Header Level of Talbe of Contents wiki macros")
	uploadWikiContentCmd.Flags().BoolVarP(&uploadWikiContentCmdFlagFixRefs, "fix-refs", "", false, "Fix relative references")
	cobra.CheckErr(uploadWikiContentCmd.MarkFlagRequired("target"))
	cobra.CheckErr(uploadWikiContentCmd.MarkFlagRequired("file"))
	cobra.CheckErr(uploadWikiContentCmd.MarkFlagFilename("file"))

	queryWikiPagesCmd.Flags().BoolVarP(&queryWikiPagesCmdFlagShowContent, "---content", "c", false, "Show pages content")
	queryWikiPagesCmd.Flags().BoolVarP(&queryWikiPagesCmdFlagShowContent, "--id", "i", false, "Show pages id")
}

func moveWikiPagesCommand() error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(moveWikiCmdFlagMovingPages)).WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}

	for _, page := range moveWikiCmdFlagMovingPages {
		progressbar.UpdateTitle(fmt.Sprintf("Moving... %v", page))

		err := api.MovePage(moveWikiCmdFlagPagesSpaceKey, page, moveWikiCmdFlagNewParentPage)
		if err != nil {
			pterm.Error.Println(fmt.Sprintf("NOT MOVED %v: %s", page, err.Error()))
		} else {
			pterm.Success.Println(fmt.Sprintf("MOVED %v", page))
		}
	}

	_, _ = progressbar.Stop()

	return err
}

func uploadWikiPageContentCommand() error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	content, err := os.ReadFile(uploadWikiContentCmdFlagSourcePath)
	if err != nil {
		return err
	}

	data := string(content)
	dataType := uploadWikiContentCmdFlagContentType

	if uploadWikiContentCmdFlagFixRefs {
		page, err := api.GetPageByID(strconv.Itoa(int(uploadWikiContentCmdFlagTargetID)))
		if err != nil {
			return err
		}

		r := regexp.MustCompile(`(\<a\shref="#)(.+?)("\>)(.+?)(\</a\>)`)
		data = r.ReplaceAllString(data, fmt.Sprintf(`${1}%s-${4}${3}${4}${5}`, page.Title))
	}

	if uploadWikiContentCmdFlagContentType == "md" || uploadWikiContentCmdFlagContentType == "markdown" {
		data = `` +
			`<ac:structured-macro ac:name="markdown" ac:schema-version="1" ac:macro-id="` + uuid.NewString() + `"><ac:parameter ac:name="atlassian-macro-output-type">INLINE</ac:parameter><ac:plain-text-body><![CDATA[` +
			string(content) +
			`]]></ac:plain-text-body></ac:structured-macro>`
		dataType = "storage"
	}

	if uploadWikiContentCmdFlagAddTableOfContents {
		data = `<ac:structured-macro xmlns:ac="http://atlassian.com/content" ac:name="expand" ac:schema-version="1" ac:macro-id="` + uuid.NewString() + `">
			<ac:parameter ac:name="title">Table of Contents</ac:parameter>
			<ac:rich-text-body>
				<p>
					<ac:structured-macro ac:name="toc" ac:schema-version="1" ac:macro-id="` + uuid.NewString() + `">
						<ac:parameter ac:name="maxLevel">` + strconv.Itoa(int(uploadWikiContentCmdFlagHeaderLevel)) + `</ac:parameter>
					</ac:structured-macro>
				</p>
			</ac:rich-text-body>
		</ac:structured-macro>
		` + data
	}

	return api.UploadContent(uploadWikiContentCmdFlagTargetID, data, dataType)
}

func getWikiPageContentCommand(pageID string) error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	p, err := api.GetContentByID(pageID, goconfluence.ContentQuery{
		Expand: []string{
			"body.storage",
			"space",
			"version",
		},
	})

	if err != nil {
		return err
	}

	println(p.Body.Storage.Value)

	labels, err := api.GetLabels(pageID)
	if err != nil {
		return err
	}

	labelNames := lo.Map(labels.Labels, func(label goconfluence.Label, i int) string {
		return label.Name
	})

	println()
	fmt.Printf("labels: %v\n", labelNames)

	return nil
}

func queryWikiPagesCommand(query string) error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	expand := make([]string, 0, 1)
	if queryWikiPagesCmdFlagShowContent {
		expand = append(expand, "body.storage")
	}

	qr, err := api.Search(goconfluence.SearchQuery{
		CQL:    query,
		Expand: expand,
	})

	if err != nil {
		return err
	}

	for _, p := range qr.Results {

		fmt.Printf("%v\n", p.Title)

		if queryWikiPagesCmdFlagShowContent {
			fmt.Printf("%v\n", p.Body.Storage.Value)
		}
	}

	return nil
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"tasker/wiki"

	"github.com/pterm/pterm"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	goconfluence "github.com/virtomize/confluence-go-api"
)

var (
	wikiCmd = &cobra.Command{
		Use:   "wiki",
		Short: "Manage Wiki pages",
		Long:  `Move wiki pages.`,
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

	deleteWikiCmd = &cobra.Command{
		Use:   "delete <Page ID|Title, ...>",
		Short: "Delete wiki pages",
		Long:  `Delete wiki pages.`,
		Args:  cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			deleteWikiCmdFlagDeletingPages = append(deleteWikiCmdFlagDeletingPages, args...)
			err := deleteWikiPagesCommand(cmd.Context())
			cobra.CheckErr(err)
		},
	}

	copyWikiCmd = &cobra.Command{
		Use:   "copy <Page ID|Title, ...>",
		Short: "Copy wiki pages",
		Long: `Create copy of wiki pages under new parent.
If page titles used, space key required.`,
		Args: cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			copyWikiCmdFlagMovingPages = append(copyWikiCmdFlagMovingPages, args...)

			if copyWikiCmdFlagPagesSpaceKey == "" {
				if _, err := strconv.Atoi(copyWikiCmdFlagNewParentPage); err != nil {
					cobra.CheckErr(errors.New("space key required when page titles used"))
					return
				}

				pageIDsCount := lo.CountBy(copyWikiCmdFlagMovingPages, func(page string) bool {
					_, err := strconv.Atoi(page)
					return err == nil
				})

				if pageIDsCount != len(copyWikiCmdFlagMovingPages) {
					cobra.CheckErr(errors.New("space key required when page titles used"))
					return
				}
			}

			err := copyWikiPagesCommand()
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
		Use:   "query [query]",
		Short: "Query wiki pages",
		Long:  `Retrieve wiki pages by query.`,
		Args:  cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			var query string
			if len(args) > 0 {
				query = args[0]
			}
			err := queryWikiPagesCommand(query)
			cobra.CheckErr(err)
		},
	}

	workitemsFromWikiPageCmd = &cobra.Command{
		Use:     "workitems <Page ID|Title, ...>",
		Aliases: []string{"wi"},
		Short:   "Extract TFS workitems",
		Long:    `Extract TFS workitems from wiki pages.`,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := workitemsFromWikiPageCommand(args)
			cobra.CheckErr(err)
		},
	}

	deleteWikiCmdFlagDeletingPages []string
	deleteWikiCmdFlagDeleteChild   bool

	moveWikiCmdFlagNewParentPage string
	moveWikiCmdFlagMovingPages   []string
	moveWikiCmdFlagPagesSpaceKey string

	copyWikiCmdFlagNewParentPage string
	copyWikiCmdFlagMovingPages   []string
	copyWikiCmdFlagPagesSpaceKey string

	uploadWikiContentCmdFlagTargetID           uint
	uploadWikiContentCmdFlagSourcePath         string
	uploadWikiContentCmdFlagContentType        string
	uploadWikiContentCmdFlagAddTableOfContents bool
	uploadWikiContentCmdFlagHeaderLevel        uint
	uploadWikiContentCmdFlagFixRefs            bool
	uploadWikiContentCmdFlagDeleteChildren     bool
	uploadWikiContentCmdFlagUploadRefs         bool

	getWikiContentCmdFlagContentType string

	queryWikiPagesCmdFlagSpace    string
	queryWikiPagesCmdFlagParent   string
	queryWikiPagesCmdFlagLabels   []string
	queryWikiPagesCmdFlagLimit    int
	queryWikiPagesCmdFlagLabelsOr bool
	queryWikiPagesCmdFlagShowID   bool

	workitemsFromWikiPageCmdFlagFirst bool
	workitemsFromWikiPageCmdFlagSpace string
)

func init() {
	rootCmd.AddCommand(wikiCmd)
	wikiCmd.AddCommand(moveWikiCmd)
	wikiCmd.AddCommand(uploadWikiContentCmd)
	wikiCmd.AddCommand(getWikiContentCmd)
	wikiCmd.AddCommand(queryWikiPagesCmd)
	wikiCmd.AddCommand(workitemsFromWikiPageCmd)
	wikiCmd.AddCommand(copyWikiCmd)
	wikiCmd.AddCommand(deleteWikiCmd)

	deleteWikiCmd.Flags().BoolVarP(&deleteWikiCmdFlagDeleteChild, "del-child", "", false, "Delete child pages")

	moveWikiCmd.Flags().StringVarP(&moveWikiCmdFlagNewParentPage, "target", "t", "", "ID or title of target parent Wiki page")
	moveWikiCmd.Flags().StringSliceVarP(&moveWikiCmdFlagMovingPages, "page", "p", nil, "ID or title of moving page")
	moveWikiCmd.Flags().StringVarP(&moveWikiCmdFlagPagesSpaceKey, "space", "s", "", "Space Key of pages")
	cobra.CheckErr(moveWikiCmd.MarkFlagRequired("target"))

	uploadWikiContentCmd.Flags().UintVarP(&uploadWikiContentCmdFlagTargetID, "target", "t", 0, "ID of target Wiki page")
	uploadWikiContentCmd.Flags().StringVarP(&uploadWikiContentCmdFlagSourcePath, "file", "f", "", "Path to file with wiki markup")
	uploadWikiContentCmd.Flags().StringVarP(&uploadWikiContentCmdFlagContentType, "type", "", "wiki", "Content type (wiki, storage, editor, md, etc.)")
	uploadWikiContentCmd.Flags().BoolVarP(&uploadWikiContentCmdFlagAddTableOfContents, "add-table-of-contents", "", false, "Prepend content with 'Table of Contents' wiki macros")
	uploadWikiContentCmd.Flags().UintVarP(&uploadWikiContentCmdFlagHeaderLevel, "header-level", "", 2, "Max Header Level of Table of Contents wiki macros")
	uploadWikiContentCmd.Flags().BoolVarP(&uploadWikiContentCmdFlagFixRefs, "fix-refs", "", false, "Fix relative references")
	uploadWikiContentCmd.Flags().BoolVarP(&uploadWikiContentCmdFlagDeleteChildren, "del-child", "", false, "Delete children of target page before upload")
	uploadWikiContentCmd.Flags().BoolVarP(&uploadWikiContentCmdFlagUploadRefs, "upload-refs", "", false, "Upload referenced files")
	cobra.CheckErr(uploadWikiContentCmd.MarkFlagRequired("target"))
	cobra.CheckErr(uploadWikiContentCmd.MarkFlagRequired("file"))
	cobra.CheckErr(uploadWikiContentCmd.MarkFlagFilename("file"))

	getWikiContentCmd.Flags().StringVarP(&getWikiContentCmdFlagContentType, "type", "", "wiki", "Content type (wiki, storage, editor, md, etc.)")

	queryWikiPagesCmd.Flags().StringVarP(&queryWikiPagesCmdFlagSpace, "space", "s", "", "Space key")
	queryWikiPagesCmd.Flags().StringVarP(&queryWikiPagesCmdFlagParent, "parent", "p", "", "Parent page id or title")
	queryWikiPagesCmd.Flags().StringArrayVarP(&queryWikiPagesCmdFlagLabels, "label", "l", nil, "Page labels")
	queryWikiPagesCmd.Flags().IntVarP(&queryWikiPagesCmdFlagLimit, "limit", "", 0, "Results limit")
	queryWikiPagesCmd.Flags().BoolVarP(&queryWikiPagesCmdFlagLabelsOr, "labels-or", "", false, "ORing labels")
	queryWikiPagesCmd.Flags().BoolVarP(&queryWikiPagesCmdFlagShowID, "id", "", false, "Show pages ID")

	workitemsFromWikiPageCmd.Flags().BoolVarP(&workitemsFromWikiPageCmdFlagFirst, "first", "f", false, "Only first task from page")
	workitemsFromWikiPageCmd.Flags().StringVarP(&workitemsFromWikiPageCmdFlagSpace, "space", "s", "", "Space Key of pages")

	copyWikiCmd.Flags().StringVarP(&copyWikiCmdFlagNewParentPage, "target", "t", "", "ID or title of target parent Wiki page")
	copyWikiCmd.Flags().StringSliceVarP(&copyWikiCmdFlagMovingPages, "page", "p", nil, "ID or title of moving page")
	copyWikiCmd.Flags().StringVarP(&copyWikiCmdFlagPagesSpaceKey, "space", "s", "", "Space Key of pages")
	cobra.CheckErr(copyWikiCmd.MarkFlagRequired("target"))
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

func deleteWikiPagesCommand(ctx context.Context) error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(deleteWikiCmdFlagDeletingPages)).WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}

	for _, pageID := range deleteWikiCmdFlagDeletingPages {
		progressbar.UpdateTitle(fmt.Sprintf("Deleting... %v", pageID))

		err = deleteWikiPage(api, pageID, deleteWikiCmdFlagDeleteChild)
		if err != nil {
			pterm.Error.Println(fmt.Sprintf("NOT DELETED %v: %s", pageID, err.Error()))
		} else {
			pterm.Success.Println(fmt.Sprintf("DELETED %v", pageID))
		}
	}

	_, _ = progressbar.Stop()

	return err
}

func deleteWikiPage(api *wiki.API, pageID string, deleteChildPages bool) error {
	childPages, err := api.GetChildPages(pageID)
	if err != nil {
		return err
	}

	if len(childPages.Results) > 0 && !deleteChildPages {
		return fmt.Errorf("page %s has children", pageID)
	}

	for _, childPage := range childPages.Results {
		err = deleteWikiPage(api, childPage.ID, deleteChildPages)
		if err != nil {
			return err
		}
	}

	_, err = api.DelContent(pageID)

	return err
}

func copyWikiPagesCommand() error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(copyWikiCmdFlagMovingPages)).WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}

	for _, page := range copyWikiCmdFlagMovingPages {
		progressbar.UpdateTitle(fmt.Sprintf("Copying... %v", page))

		err := api.CopyPage(copyWikiCmdFlagPagesSpaceKey, page, copyWikiCmdFlagNewParentPage)
		if err != nil {
			pterm.Error.Println(fmt.Sprintf("NOT COPIED %v: %s", page, err.Error()))
		} else {
			pterm.Success.Println(fmt.Sprintf("COPIED %v", page))
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

	if uploadWikiContentCmdFlagDeleteChildren {
		childPages, err := api.GetChildPages(strconv.Itoa(int(uploadWikiContentCmdFlagTargetID)))
		if err != nil {
			return fmt.Errorf("failed to fetch child pages: %w", err)
		}

		for _, cp := range childPages.Results {
			err = deleteWikiPage(api, cp.ID, true)
			if err != nil {
				return fmt.Errorf("failed to delete child page %v: %w", cp, err)
			}
		}
	}

	opts := []wiki.UploadOption{
		wiki.AddTableOfContents(uploadWikiContentCmdFlagAddTableOfContents),
		wiki.HeaderLevel(int(uploadWikiContentCmdFlagHeaderLevel)),
	}

	// if uploadWikiContentCmdFlagUploadRefs && wiki.IsMarkdownContentType(uploadWikiContentCmdFlagContentType) {
	// 	u, err := mdtree.CreateWikiContentUploader(
	// 		api,
	// 		uploadWikiContentCmdFlagSourcePath,
	// 		strconv.Itoa(int(uploadWikiContentCmdFlagTargetID)),
	// 	)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return u.Upload()
	// }

	return wiki.UploadContent(api, strconv.Itoa(int(uploadWikiContentCmdFlagTargetID)), data, dataType, opts...)
}

func getWikiPageContentCommand(pageID string) error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	expand := []string{
		"space",
		"version",
	}

	if getWikiContentCmdFlagContentType != "" {
		expand = append(expand, "body."+getWikiContentCmdFlagContentType)
	} else {
		expand = append(expand, "body.storage")
	}

	p, err := api.GetContentByID(pageID, goconfluence.ContentQuery{
		Expand: expand,
	})

	if err != nil {
		return err
	}

	if p.Body.View != nil && p.Body.View.Value != "" {
		println(p.Body.View.Value)
	} else {
		println(p.Body.Storage.Value)
	}

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

	if query != "" {
		query = "(" + query + ")"
	}

	filterFunc := func(operator, query, filter string) string {
		if query != "" {
			query += " " + operator + " "
		}
		query += filter
		return query
	}

	and := func(query, filter string) string { return filterFunc("AND", query, filter) }
	or := func(query, filter string) string { return filterFunc("OR", query, filter) }

	if !strings.Contains(query, "type=") {
		query = and(query, "type=page")
	}

	if queryWikiPagesCmdFlagSpace != "" {
		query = and(query, "space=\""+queryWikiPagesCmdFlagSpace+"\"")
	}

	if queryWikiPagesCmdFlagParent != "" {
		parentID := queryWikiPagesCmdFlagParent
		if _, err := strconv.Atoi(parentID); err != nil {
			p, err := api.GetPageByTitle(queryWikiPagesCmdFlagParent, queryWikiPagesCmdFlagSpace)
			if err != nil {
				return err
			}

			parentID = p.ID
		}

		query = and(query, "parent="+parentID)
	}

	labelsFilter := ""
	for _, label := range queryWikiPagesCmdFlagLabels {
		if queryWikiPagesCmdFlagLabelsOr {
			labelsFilter = or(labelsFilter, "label=\""+label+"\"")
		} else {
			labelsFilter = and(labelsFilter, "label=\""+label+"\"")
		}
	}

	if labelsFilter != "" {
		query = and(query, "("+labelsFilter+")")
	}

	qr, err := api.SearchContent(goconfluence.SearchQuery{
		CQL:   query,
		Limit: queryWikiPagesCmdFlagLimit,
	})

	if err != nil {
		return err
	}

	for _, p := range qr.Results {
		if queryWikiPagesCmdFlagShowID {
			fmt.Printf("%v\n", p.ID)
		} else {
			fmt.Printf("%v\n", p.Title)
		}
	}

	return nil
}

func workitemsFromWikiPageCommand(pages []string) error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	for _, page := range pages {
		content, err := api.GetPageByTitle(page, workitemsFromWikiPageCmdFlagSpace, wiki.GetPageByTitleWithBody())
		if err != nil {
			return err
		}

		tasks, err := wiki.ParseTfsTasks(content)
		if err != nil {
			return err
		}

		for _, task := range tasks {
			fmt.Printf("%v\n", task.ItemID)
			if workitemsFromWikiPageCmdFlagFirst {
				break
			}
		}
	}

	return nil
}

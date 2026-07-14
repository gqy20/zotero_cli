package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
	"zotero_cli/internal/config"
)

type payloadFlags struct {
	data   string
	from   string
	set    []string
	name   string
	parent string
	text   string
}

func (c *CLI) writeService() app.WriteService { return app.NewWriteService() }

func addPayloadFlags(cmd *cobra.Command, flags *payloadFlags) {
	cmd.Flags().StringVar(&flags.data, "data", "", "complete JSON object")
	cmd.Flags().StringVar(&flags.from, "from", "", "read JSON from a file, or - for stdin")
	cmd.Flags().StringArrayVar(&flags.set, "set", nil, "set FIELD=VALUE (repeatable)")
}

func addSafetyFlags(cmd *cobra.Command, safety *app.SafetyOptions, deletion bool) {
	cmd.Flags().BoolVar(&safety.DryRun, "dry-run", false, "validate and show the planned operation without writing")
	cmd.Flags().IntVar(&safety.IfVersion, "if-version", 0, "require this library version; omitted resolves the current version")
	if deletion {
		cmd.Flags().BoolVarP(&safety.Yes, "yes", "y", false, "confirm destructive operation")
	}
}

func (c *CLI) resolvePayload(flags payloadFlags, defaults map[string]any) (map[string]any, error) {
	hasGeneric := strings.TrimSpace(flags.data) != "" || strings.TrimSpace(flags.from) != "" || len(flags.set) > 0
	if !hasGeneric {
		if len(defaults) == 0 {
			return nil, fmt.Errorf("one of --data, --from, or --set is required")
		}
		return defaults, nil
	}
	if len(defaults) > 0 {
		return nil, fmt.Errorf("resource-specific input cannot be combined with --data, --from, or --set")
	}
	return app.ResolvePayload(app.PayloadInput{Data: flags.data, From: flags.from, Set: flags.set}, c.stdin)
}

func (c *CLI) runWrite(cmd *cobra.Command, opts *globalOptions, path app.CommandPath, run func(context.Context, app.WriteService) (app.Result, error)) error {
	tasteWarning := libraryTasteWriteWarning(path)
	err := c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
		result, err := run(ctx, c.writeService())
		if err == nil && tasteWarning != nil {
			result.Warnings = append(result.Warnings, *tasteWarning)
		}
		return result, err
	})
	if errors.Is(err, app.ErrCancelled) {
		return &exitError{code: 130, err: app.ErrCancelled}
	}
	return err
}

func libraryTasteWriteWarning(path app.CommandPath) *app.Warning {
	relevant := path.Resource == "tag" ||
		(path.Resource == "item" && (path.Action == "tag" || path.Action == "untag")) ||
		(path.Resource == "coll" && path.Action != "delete")
	if !relevant {
		return nil
	}
	cfg, configPath, err := config.Load()
	if err != nil {
		return nil
	}
	taste, err := app.LoadLibraryTaste(cfg, configPath)
	if err != nil || taste.Exists {
		return nil
	}
	return &app.Warning{Code: "taste_missing", Message: "library taste is not configured; run `zot lib taste init` before classification changes"}
}

func (c *CLI) addObjectWriteCommands(resource *cobra.Command, opts *globalOptions, kind string) {
	label := kind
	if kind == "coll" {
		label = "collection"
	}
	article := "a "
	if kind == "item" {
		article = "an "
	}
	var createPayload payloadFlags
	var createSafety app.SafetyOptions
	create := &cobra.Command{Use: "new", Short: "Create " + article + label, Args: cobra.NoArgs}
	addPayloadFlags(create, &createPayload)
	addSafetyFlags(create, &createSafety, false)
	if kind == "coll" {
		create.Flags().StringVar(&createPayload.name, "name", "", "collection name")
		create.Flags().StringVar(&createPayload.parent, "parent", "", "parent collection key")
	}
	if kind == "note" {
		create.Flags().StringVar(&createPayload.parent, "parent", "", "parent item key")
		create.Flags().StringVar(&createPayload.text, "text", "", "note text or HTML")
	}
	create.RunE = func(cmd *cobra.Command, _ []string) error {
		defaults := map[string]any{}
		switch kind {
		case "coll":
			if createPayload.name != "" {
				defaults["name"] = createPayload.name
				if createPayload.parent != "" {
					defaults["parentCollection"] = createPayload.parent
				}
			}
		case "note":
			if createPayload.parent != "" || createPayload.text != "" {
				if createPayload.parent == "" || createPayload.text == "" {
					return &exitError{code: ExitUsage, err: fmt.Errorf("--parent and --text are required together")}
				}
				defaults = map[string]any{"itemType": "note", "parentItem": createPayload.parent, "note": createPayload.text}
			}
		}
		payload, err := c.resolvePayload(createPayload, defaults)
		if err != nil {
			return &exitError{code: ExitUsage, err: err}
		}
		path := app.CommandPath{Resource: kind, Action: "new"}
		return c.runWrite(cmd, opts, path, func(ctx context.Context, service app.WriteService) (app.Result, error) {
			return service.Create(ctx, kind, app.ObjectWriteRequest{Payload: payload, Safety: createSafety})
		})
	}

	var editPayload payloadFlags
	var editSafety app.SafetyOptions
	edit := &cobra.Command{Use: "edit KEY", Short: "Edit " + article + label, Args: cobra.ExactArgs(1)}
	addPayloadFlags(edit, &editPayload)
	addSafetyFlags(edit, &editSafety, false)
	if kind == "coll" {
		edit.Flags().StringVar(&editPayload.name, "name", "", "collection name")
		edit.Flags().StringVar(&editPayload.parent, "parent", "", "parent collection key")
	}
	if kind == "note" {
		edit.Flags().StringVar(&editPayload.text, "text", "", "note text or HTML")
	}
	edit.RunE = func(cmd *cobra.Command, args []string) error {
		defaults := map[string]any{}
		if kind == "coll" && editPayload.name != "" {
			defaults["name"] = editPayload.name
			if editPayload.parent != "" {
				defaults["parentCollection"] = editPayload.parent
			}
		}
		if kind == "note" && editPayload.text != "" {
			defaults["note"] = editPayload.text
		}
		payload, err := c.resolvePayload(editPayload, defaults)
		if err != nil {
			return &exitError{code: ExitUsage, err: err}
		}
		path := app.CommandPath{Resource: kind, Action: "edit"}
		return c.runWrite(cmd, opts, path, func(ctx context.Context, service app.WriteService) (app.Result, error) {
			return service.Update(ctx, kind, app.ObjectWriteRequest{Keys: args, Payload: payload, Safety: editSafety})
		})
	}

	var deleteSafety app.SafetyOptions
	remove := &cobra.Command{Use: "delete KEY...", Short: "Permanently delete " + label + " objects", Args: cobra.MinimumNArgs(1)}
	addSafetyFlags(remove, &deleteSafety, true)
	remove.RunE = func(cmd *cobra.Command, args []string) error {
		deleteSafety.Confirm = c.confirm
		path := app.CommandPath{Resource: kind, Action: "delete"}
		return c.runWrite(cmd, opts, path, func(ctx context.Context, service app.WriteService) (app.Result, error) {
			return service.Delete(ctx, kind, app.ObjectWriteRequest{Keys: args, Safety: deleteSafety})
		})
	}
	resource.AddCommand(create, edit, remove)
}

func (c *CLI) addItemWriteCommands(item *cobra.Command, opts *globalOptions) {
	c.addObjectWriteCommands(item, opts, "item")
	var importDryRun bool
	var importCollection string
	importCmd := &cobra.Command{Use: "import PATH", Short: "Import a PDF into Zotero desktop", Args: cobra.ExactArgs(1)}
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "validate the PDF and Zotero connection without importing")
	importCmd.Flags().StringVar(&importCollection, "collection", "", "collection key, unique name, or full path")
	importCmd.RunE = func(cmd *cobra.Command, args []string) error {
		service := app.NewItemImportService()
		path := app.CommandPath{Resource: "item", Action: "import"}
		return c.renderResult(cmd.Context(), opts, path, func(ctx context.Context) (app.Result, error) {
			return service.Import(ctx, app.ItemImportRequest{Path: args[0], Collection: importCollection, DryRun: importDryRun})
		})
	}
	item.AddCommand(importCmd)
	for _, spec := range []struct {
		name string
		add  bool
	}{{"tag", true}, {"untag", false}} {
		var tag string
		var safety app.SafetyOptions
		short := "Add a tag to library items"
		if !spec.add {
			short = "Remove a tag from library items"
		}
		cmd := &cobra.Command{Use: spec.name + " KEY...", Short: short, Args: cobra.MinimumNArgs(1)}
		cmd.Flags().StringVar(&tag, "tag", "", "tag name")
		addSafetyFlags(cmd, &safety, false)
		add := spec.add
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(tag) == "" {
				return &exitError{code: ExitUsage, err: fmt.Errorf("--tag is required")}
			}
			path := app.CommandPath{Resource: "item", Action: cmd.Name()}
			return c.runWrite(cmd, opts, path, func(ctx context.Context, service app.WriteService) (app.Result, error) {
				return service.Tags(ctx, app.TagWriteRequest{Keys: args, Tag: tag, Add: add, Safety: safety})
			})
		}
		item.AddCommand(cmd)
	}
}

func (c *CLI) addCollectionWriteCommands(coll *cobra.Command, opts *globalOptions) {
	c.addObjectWriteCommands(coll, opts, "coll")
	for _, spec := range []struct {
		name string
		add  bool
	}{{"add", true}, {"remove", false}} {
		var safety app.SafetyOptions
		cmd := &cobra.Command{Use: spec.name + " COLLKEY ITEMKEY...", Short: spec.name + " items in a collection", Args: cobra.MinimumNArgs(2)}
		addSafetyFlags(cmd, &safety, false)
		add := spec.add
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			path := app.CommandPath{Resource: "coll", Action: cmd.Name()}
			return c.runWrite(cmd, opts, path, func(ctx context.Context, service app.WriteService) (app.Result, error) {
				return service.Membership(ctx, app.MembershipRequest{CollectionKey: args[0], ItemKeys: args[1:], Add: add, Safety: safety})
			})
		}
		coll.AddCommand(cmd)
	}
}

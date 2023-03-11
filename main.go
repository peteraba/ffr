package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitfield/script"
	cli "github.com/urfave/cli/v2"
)

var process = func(c *cli.Context, argCount int, fn func(*cli.Context, []string, os.FileInfo, bool, bool) error) error {
	args := c.Args().Slice()
	dryRun := c.Bool("dryRun")
	verbose := c.Bool("verbose")

	if argCount > len(args) {
		argCount = len(args)
	}

	filePaths := args[argCount:]
	args = args[:argCount]

	for _, filePath := range filePaths {
		fi, err := os.Stat(filePath)
		if err != nil {
			if verbose {
				log.Printf("argument is not a file: %s\n", filePath)
			}

			continue
		}

		if fi.IsDir() {
			if verbose {
				log.Printf("file is a directory: %s\n", filePath)
			}

			continue
		}

		err = fn(c, args, fi, dryRun, verbose)
		if err != nil {
			log.Fatalln(err)
		}
	}

	return nil
}

var exec = func(command string) (string, error) {
	p := script.Exec(command)
	output, err := p.String()
	if err != nil {
		log.Println(err)
	}

	return output, err
}

var reEncode = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()

	codec := c.String("codec")
	crf := c.Int("crf")

	var command string
	switch codec {
	case "libx264":
		command = fmt.Sprintf(`ffmpeg -i "%s" -c:v libx264 -crf %d -c:a aac -q:a 100 "%s.mp4"`, filePath, crf, filePath)
		break
	case "vp9":
		command = fmt.Sprintf(`ffmpeg -i "%s" -c:v vp9 -c:a aac "%s.mkv"`, filePath, filePath)
	default:
		return fmt.Errorf("unsupported codec")
	}

	if dryRun || verbose {
		fmt.Println(command)
	}

	if dryRun {
		return nil
	}

	output, err := exec(command)
	if verbose {
		log.Println(output)
	}

	return err
}

var prefix = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()
	if len(args) == 0 {
		return nil
	}

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	newPath := args[0] + basePath + ext
	if c.Bool("separate") {
		newPath = args[0] + "-" + basePath + ext
	}

	if verbose || dryRun {
		log.Println(filePath, " -> ", newPath)
	}

	if dryRun {
		return nil
	}

	return os.Rename(filePath, newPath)
}

var suffix = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()
	if len(args) == 0 {
		return nil
	}

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	newPath := basePath + args[0] + ext
	if c.Bool("separate") {
		newPath = basePath + "-" + args[0] + ext
	}

	if verbose || dryRun {
		log.Println(filePath, " -> ", newPath)
	}

	if dryRun {
		return nil
	}

	return os.Rename(filePath, newPath)
}

var replace = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()
	if len(args) < 2 {
		return nil
	}

	old := args[0]
	new := args[1]

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	newPath := strings.Replace(basePath+ext, old, new, 1)

	if verbose || dryRun {
		log.Printf(`"%s" -> "%s", old: "%s", new: "%s"`, filePath, newPath, old, new)
	}

	if dryRun {
		return nil
	}

	return os.Rename(filePath, newPath)
}

var merge = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()
	r := regexp.MustCompile(`-(\d+)(\w+)(-[a-z23-]+)?$`)

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	type d struct {
		num  string
		text string
	}

	var descriptions []d
	for {
		matches := r.FindStringSubmatch(basePath)
		if len(matches) == 0 {
			break
		}

		basePath = basePath[:len(basePath)-len(matches[0])]
		descriptions = append(descriptions, d{num: matches[1], text: matches[2]})
	}

	if len(descriptions) == 0 {
		if verbose {
			log.Printf("found nothing to merge in '%s'", basePath)
		}

		return nil
	}

	keep := c.Int("keep")
	if keep < 1 {
		keep = len(descriptions) - 1
	} else {
		keep--
	}

	if len(descriptions) < keep {
		return fmt.Errorf("can't find description #%d in '%s'", keep, basePath)
	}

	newPath := basePath + "-" + descriptions[keep].num + descriptions[0].text + ext

	if verbose || dryRun {
		log.Println(filePath, " -> ", newPath)
	}

	if dryRun {
		return nil
	}

	return os.Rename(filePath, newPath)
}

var insertBefore = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()
	if len(args) < 1 {
		return nil
	}

	regularExpression := args[0]
	insert := args[1]

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}
	newPath := basePath + insert + ext

	r, err := regexp.Compile(`-\d*` + regularExpression + `.*$`)
	if err != nil {
		return fmt.Errorf("regexp failed, err: %w", err)
	}
	matched := r.FindString(basePath)
	if matched != "" {
		newPath = strings.Replace(basePath, matched, insert+matched, 1) + ext
	}

	if verbose || dryRun {
		log.Printf(`"%s" -> "%s", old: "%s", new: "%s"`, filePath, newPath, matched, insert+matched)
	}

	if dryRun {
		return nil
	}

	return os.Rename(filePath, newPath)
}

var insertDimensionsBefore = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	cmd := fmt.Sprintf(`ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=s=x:p=0 '%s'`, fi.Name())

	output, err := exec(cmd)
	if err != nil {
		return fmt.Errorf("failed to probe file. command: '%s', err: %w", cmd, err)
	}

	output = strings.TrimSpace(output)
	if verbose {
		log.Printf("dimenensions found. file: '%s', dimensions: '%s'", fi.Name(), output)
	}

	return insertBefore(c, append(args, `-`+output), fi, dryRun, verbose)
}

func main() {
	app := &cli.App{
		Name: "ffreencode",
		Commands: []*cli.Command{
			{
				Name:      "reencode",
				Usage:     "reencode a file via ffmpeg",
				ArgsUsage: "[files...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "dryRun",
						Aliases: []string{"d"},
						Value:   false,
						Usage:   "only print them, do not execute anything",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Value:   false,
						Usage:   "print commands before executing them",
					},
					&cli.StringFlag{
						Name:  "codec",
						Usage: "codec to use for encoding [libx264, vp9]",
						Value: "vp9",
					},
					&cli.IntFlag{
						Name:  "crf",
						Usage: "crf to use for encoding [https://slhck.info/video/2017/02/24/crf-guide.html]",
						Value: 23,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, reEncode)
				},
			},
			{
				Name:      "prepend",
				Aliases:   []string{"p", "prefix"},
				Usage:     "prefix file names with a fixed string",
				ArgsUsage: "[text to insert] [files...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "dryRun",
						Aliases: []string{"d"},
						Value:   false,
						Usage:   "only print them, do not execute anything",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Value:   false,
						Usage:   "print commands before executing them",
					},
					&cli.BoolFlag{
						Name:    "separate",
						Aliases: []string{"s"},
						Usage:   "separate suffix with a dash",
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, prefix)
				},
			},
			{
				Name:      "append",
				Aliases:   []string{"a", "suffix", "s"},
				Usage:     "suffix file names with a fixed string",
				ArgsUsage: "[text to insert] [files...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "dryRun",
						Aliases: []string{"d"},
						Value:   false,
						Usage:   "only print them, do not execute anything",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Value:   false,
						Usage:   "print commands before executing them",
					},
					&cli.BoolFlag{
						Name:    "separate",
						Aliases: []string{"s"},
						Usage:   "separate suffix with a dash",
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, suffix)
				},
			},
			{
				Name:      "replace",
				Usage:     "replace a fixed string in file names",
				ArgsUsage: "[needle] [text to insert] [files...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "dryRun",
						Aliases: []string{"d"},
						Value:   false,
						Usage:   "only print them, do not execute anything",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Value:   false,
						Usage:   "print commands before executing them",
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 2, replace)
				},
			},
			{
				Name:      "merge",
				Aliases:   []string{"m"},
				Usage:     "merge the generated descriptions [foo-12ffc-1bar -> abc-12bar]",
				ArgsUsage: "[files...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "dryRun",
						Aliases: []string{"d"},
						Value:   false,
						Usage:   "only print them, do not execute anything",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Value:   false,
						Usage:   "print commands before executing them",
					},
					&cli.IntFlag{
						Name:    "keep",
						Aliases: []string{"k"},
						Value:   0,
						Usage:   "number of description to keep [0 = last]",
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, merge)
				},
			},
			{
				Name:      "insert-before",
				Aliases:   []string{"ib"},
				Usage:     "insert before the generated descriptions",
				ArgsUsage: "[regular expression] [text to insert] [files...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "dryRun",
						Aliases: []string{"d"},
						Value:   false,
						Usage:   "only print them, do not execute anything",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Value:   false,
						Usage:   "print commands before executing them",
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 2, insertBefore)
				},
			},
			{
				Name:      "insert-dimensions",
				Aliases:   []string{"id"},
				Usage:     "insert video dimensions before the generated descriptions",
				ArgsUsage: "[regular expression] [files...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "dryRun",
						Aliases: []string{"d"},
						Value:   false,
						Usage:   "only print them, do not execute anything",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Value:   false,
						Usage:   "print commands before executing them",
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, insertDimensionsBefore)
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

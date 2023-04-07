package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bitfield/script"
	cli "github.com/urfave/cli/v2"
)

// commands
const (
	reencodeCommand     = "reencode"
	reencodeUsage       = "reencode a file via ffmpeg"
	reencodeArgsUsage   = "[files...]"
	reencodeDescription = `
Find more about the various codecs and their settings here:
https://trac.ffmpeg.org/wiki/Encode/H.265
https://trac.ffmpeg.org/wiki/Encode/H.264
https://trac.ffmpeg.org/wiki/Encode/VP9`

	keyFramesCommand   = "keyframes"
	keyFramesAliases   = "k"
	keyFramesUsage     = "list keyframes of video file(s)"
	keyFramesArgsUsage = "[files...]"

	prefixCommand   = "prefix"
	prefixAliases   = "p"
	prefixUsage     = "prefix file names with a fixed string"
	prefixArgsUsage = "[text to insert] [files...]"

	suffixCommand   = "suffix"
	suffixAliases   = "s"
	suffixUsage     = "suffix file names with a fixed string"
	suffixArgsUsage = "[text to insert] [files...]"

	replaceCommand   = "replace"
	replaceAliases   = "r"
	replaceUsage     = "replace a fixed string in file names"
	replaceArgsUsage = "[needle] [text to insert] [files...]"

	mergePartsCommand   = "merge-parts"
	mergePartsAliases   = "m"
	mergePartsUsage     = "merge the generated descriptions [foo-12ffc-1bar -> abc-12bar]"
	mergePartsArgsUsage = "[files...]"

	addNumberCommand   = "add-number"
	addNumberAliases   = "a"
	addNumberUsage     = "add a number to the last number found in the file"
	addNumberArgsUsage = "[number-to-addNumber] [files...]"

	deletePartsCommand   = "delete-parts"
	deletePartsAliases   = "d"
	deletePartsUsage     = "delete certain parts"
	deletePartsArgsUsage = "[comma-separated-list] [files...]"

	insertBeforeCommand   = "insert-before"
	insertBeforeAliases   = "ib"
	insertBeforeUsage     = "insert before the generated descriptions"
	insertBeforeArgsUsage = "[text to insert] [files...]"

	insertDimensionsCommand   = "insert-dimensions"
	insertDimensionsAliases   = "id"
	insertDimensionsUsage     = "insert video dimensions before the generated descriptions"
	insertDimensionsArgsUsage = "[files...]"
)

// flags
const (
	dryRunFlag  = "dryRun"
	dryRunAlias = "d"
	dryRunUsage = "only print commands, do not execute anything"

	verboseFlag  = "verbose"
	verboseAlias = "v"
	verboseUsage = "print commands before executing them"

	codecFlag  = "codec"
	codecUsage = "codec to use for encoding [libx264, libx265, vp9]"

	crfFlag  = "crf"
	crfUsage = "crf to use for encoding [https://slhck.info/video/2017/02/24/crf-guide.html]"

	presetFlag  = "preset"
	presetUsage = "preset to use for encoding [%s] (x264, x265 only)"

	skipPartsFlag  = "skip-parts"
	skipPartsAlias = "s"
	skipPartsUsage = "number of dash-separated parts to skip"

	skipFindsFlag  = "skip-finds"
	skipFindsAlias = "s"
	skipFindsUsage = "number finds to skip"

	forceFlag  = "force-overwrite"
	forceAlias = "f"
	forceUsage = "force overwriting existing files"

	regexpFlag  = "regular-expression"
	regexpAlias = "r"
	regexpUsage = "regular expression which could be used to filter parts"

	deleteFlag  = "delete"
	deleteAlias = "del"
	deleteUsage = "text to delete after merging"
)

const (
	separator = "-"
)

const (
	defaultCodec  = "libx265"
	defaultPreset = "ultrafast"
)

var (
	allowedPresets = []string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"}
)

var process = func(c *cli.Context, argCount int, fn func(*cli.Context, []string, os.FileInfo, bool, bool) error) error {
	args := c.Args().Slice()
	dryRun := c.Bool(dryRunFlag)
	verbose := c.Bool(verboseFlag)

	if argCount > len(args) {
		argCount = len(args)
	}

	filePaths := args[argCount:]
	args = args[:argCount]

	var fis []os.FileInfo

	for _, filePath := range filePaths {
		fi, err := os.Stat(filePath)
		if err != nil {
			log.Fatalf("argument is not a file: %s, err: %s", filePath, err)
		}

		if fi.IsDir() {
			log.Fatalf("file is a directory: %s", filePath)
		}

		if verbose {
			log.Printf("file is okay: %s", filePath)
		}

		fis = append(fis, fi)
	}

	if len(filePaths) == 0 {
		log.Fatalf("no files provided")

		return nil
	}

	for _, fi := range fis {
		err := fn(c, args, fi, dryRun, verbose)
		if err != nil {
			log.Println(err)
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

var keyFrames = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	command := fmt.Sprintf(`ffprobe -v error -select_streams v:0 -skip_frame nokey -show_entries frame=pkt_pts_time -of csv=p=0 "%s"`, fi.Name())

	if dryRun || verbose {
		log.Println(command)
	}

	if dryRun {
		return nil
	}

	output, err := exec(command)
	if err != nil {
		return err
	}

	maxCount := 4
	var numbers []string
	for i, line := range strings.Split(output, "\n") {
		if i >= maxCount {
			break
		}

		if line == "" {
			continue
		}

		n, err := strconv.ParseFloat(line, 32)
		if err != nil {
			return err
		}

		numbers = append(numbers, fmt.Sprintf("%.1f", n))
	}

	log.Printf("file: %s\n", fi.Name())
	log.Printf("indexes: %s...\n\n", strings.Join(numbers, ", "))

	return nil
}

var reEncode = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()

	codec := c.String(codecFlag)
	crf := c.Int(crfFlag)

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	switch codec {
	case "libx265":
		// https://trac.ffmpeg.org/wiki/Encode/H.265
		if crf == 0 {
			crf = 28
		}
		break
	case "libx264":
		// https://trac.ffmpeg.org/wiki/Encode/H.264
		if crf == 0 {
			crf = 23
		}
		break
	case "vp9":
		// https://trac.ffmpeg.org/wiki/Encode/VP9
		if crf == 0 {
			crf = 31
		}
	default:
		return fmt.Errorf("unsupported codec")
	}

	preset := c.String(presetFlag)
	found := false
	for _, p := range allowedPresets {
		if p == preset {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid preset. preset: %s", preset)
	}

	outputBasePath := fmt.Sprintf("%s-%s-%d-%s", basePath, codec, crf, preset)

	var command string
	switch codec {
	case "libx265":
		// https://trac.ffmpeg.org/wiki/Encode/H.265
		command = fmt.Sprintf(`ffmpeg -i "%s" -c:v libx265 -x265-params keyint=1 -preset %s -crf %d -c:a aac -q:a 100 -tag:v hvc1 "%s.mp4"`, filePath, preset, crf, outputBasePath)
		break
	case "libx264":
		// https://trac.ffmpeg.org/wiki/Encode/H.264
		command = fmt.Sprintf(`ffmpeg -i "%s" -c:v libx264 -x264-params keyint=1 -preset %s -crf %d -c:a aac -q:a 100 "%s.mp4"`, filePath, preset, crf, outputBasePath)
		break
	case "vp9":
		// https://trac.ffmpeg.org/wiki/Encode/VP9
		command = fmt.Sprintf(`ffmpeg -i "%s" -c:v vp9 -crf %d -b:v 0 -c:a aac "%s.mkv"`, filePath, crf, outputBasePath)
	default:
		return fmt.Errorf("unsupported codec")
	}

	if dryRun || verbose {
		log.Println(command)
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

func safeRename(oldPath, newPath string, forceOverwrite bool) error {
	if oldPath == newPath {
		return nil
	}

	_, err := os.Stat(newPath)
	if forceOverwrite || err != nil && os.IsNotExist(err) {
		return os.Rename(oldPath, newPath)
	}

	if err != nil {
		log.Printf("unexpected error during renaming file. old path: '%s', new path: '%s', err: %s", oldPath, newPath, err)
	}

	log.Printf("file already exists. new path: '%s'", newPath)

	return nil
}

func concat(parts []string, skip int, newPart, ext, separator string) string {
	start := strings.Join(parts[:skip], separator)
	end := strings.Join(parts[skip:], separator)

	if start != "" {
		start += separator
	}
	if end != "" {
		end = separator + end
	}

	return start + newPart + end + ext
}

var prefix = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()
	if len(args) == 0 {
		return nil
	}

	skip := c.Int(skipPartsFlag)
	newPart := args[0]

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	parts := strings.Split(basePath, separator)
	if skip > len(parts) {
		return fmt.Errorf("more to skip then parts present. file: '%s' skip: %d, parts: %d", basePath, skip, len(parts))
	}

	newPath := concat(parts, skip, newPart, ext, separator)

	if verbose || dryRun {
		log.Println(filePath, " -> ", newPath)
	}

	if dryRun {
		return nil
	}

	return safeRename(filePath, newPath, c.Bool(forceFlag))
}

var suffix = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()
	if len(args) == 0 {
		return nil
	}

	skip := c.Int(skipPartsFlag)
	newPart := args[0]

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	parts := strings.Split(basePath, separator)
	if skip > len(parts) {
		return fmt.Errorf("more to skip then parts present. file: '%s' skip: %d, parts: %d", basePath, skip, len(parts))
	}
	skipInverse := len(parts) - skip

	newPath := concat(parts, skipInverse, newPart, ext, separator)

	if verbose || dryRun {
		log.Println(filePath, " -> ", newPath)
	}

	if dryRun {
		return nil
	}

	return safeRename(filePath, newPath, c.Bool(forceFlag))
}

var replace = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()
	if len(args) < 2 {
		return nil
	}

	search := args[0]
	replaceWith := args[1]
	skip := c.Int(skipFindsFlag)

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	parts := strings.Split(basePath, search)
	if skip > len(parts) {
		return fmt.Errorf("more to skip then found. file: '%s', skip: %d, found: %d", basePath, skip, len(parts))
	}

	start := strings.Join(parts[:skip], search)
	end := strings.Join(parts[skip:], replaceWith)

	newPath := start + end + ext
	if len(start) > 0 && len(end) > 0 {
		newPath = start + search + end + ext
	}

	if verbose || dryRun {
		log.Printf(`"%s" -> "%s", search: "%s", replace with: "%s"`, filePath, newPath, search, replaceWith)
	}

	if dryRun {
		return nil
	}

	return safeRename(filePath, newPath, c.Bool(forceFlag))
}

var mergeParts = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	regularExpression := c.String(regexpFlag)
	if regularExpression == "" {
		regularExpression = "([a-z]+)"
	} else {
		re := strings.Replace(strings.Replace(regularExpression, "(", "", -1), ")", "", -1)
		if len(re) < len(regularExpression)-2 {
			return errors.New("wrong regular expression received")
		}
		if len(re) == len(regularExpression) {
			regularExpression = `(` + regularExpression + `)`
		}
	}

	r, err := regexp.Compile(`-(\d+)(` + regularExpression + `(-[a-z]+\d*)*)`)
	if err != nil {
		return err
	}

	matches := r.FindAllStringSubmatch(basePath, -1)
	var (
		sum   int
		extra = make([]string, len(matches))
	)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		basePath = basePath[:len(basePath)-len(m[0])]

		s, err := strconv.ParseInt(m[1], 10, 32)
		if err != nil {
			return err
		}
		sum += int(s)
		extra[i] = m[2]

		if verbose {
			log.Printf("base: %s", basePath)
			log.Printf("extra: %#v", extra)
			log.Printf("matches: %#v", m)
			log.Printf("sum: %d", sum)
			log.Println()
		}
	}

	newPath := fmt.Sprintf("%s-%d%s%s", basePath, sum, strings.Join(extra, "-"), ext)
	if c.String(deleteFlag) != "" {
		newPath = strings.Replace(newPath, c.String(deleteFlag), "", 1)
	}

	if verbose || dryRun {
		log.Printf(`"%s" -> "%s"`, filePath, newPath)
	}

	if dryRun {
		return nil
	}

	return safeRename(filePath, newPath, c.Bool(forceFlag))
}

var addNumber = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	regularExpression := c.String(regexpFlag)
	if regularExpression == "" {
		regularExpression = "([a-z][a-z]+)"
	} else {
		re := strings.Replace(strings.Replace(regularExpression, "(", "", -1), ")", "", -1)
		if len(re) < len(regularExpression)-2 {
			return errors.New("wrong regular expression received")
		}
		if len(re) == len(regularExpression) {
			regularExpression = `(` + regularExpression + `)`
		}
	}

	reFinal := `^(.+)-(\d+)(` + regularExpression + `(-.*)?)$`
	if verbose {
		log.Printf("regular expression received: %s", regularExpression)
		log.Printf("regular expression final: %s", reFinal)
	}

	r, err := regexp.Compile(reFinal)
	if err != nil {
		return err
	}

	matches := r.FindStringSubmatch(basePath)
	if verbose {
		log.Printf("basePath: %s", basePath)
		log.Printf("matches: %#v", matches)
	}

	if len(matches) == 0 {
		return errors.New("no matches")
	}

	s1, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		return err
	}

	s2, err := strconv.ParseInt(matches[2], 10, 32)
	if err != nil {
		return err
	}

	newPath := fmt.Sprintf("%s-%d%s%s", matches[1], s1+s2, matches[3], ext)

	if verbose || dryRun {
		log.Printf(`"%s" -> "%s"`, filePath, newPath)
	}

	return safeRename(filePath, newPath, c.Bool(forceFlag))
}

var deleteParts = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	return nil
}

var insertBefore = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	filePath := fi.Name()
	if len(args) < 1 {
		return nil
	}

	regularExpression := c.String(regexpFlag)
	if regularExpression == "" {
		regularExpression = "[a-z]+"
	}
	insert := args[1]

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}
	newPath := basePath + insert + ext

	r, err := regexp.Compile(`-\d+` + regularExpression + `-.*$`)
	if err != nil {
		return fmt.Errorf("regexp failed, err: %w", err)
	}
	matched := r.FindString(basePath)
	if matched != "" {
		newPath = strings.Replace(basePath, matched, insert+matched, 1) + ext
	}

	if verbose || dryRun {
		log.Printf(`"%s" -> "%s", found: "%s", new: "%s"`, filePath, newPath, matched, insert+matched)
	}

	if dryRun {
		return nil
	}

	return safeRename(filePath, newPath, c.Bool(forceFlag))
}

var wellKnown = map[string]string{
	"640x480":   "sd-480p",
	"1280x720":  "hd-720p",
	"1920x1080": "fullhd-1080p",
	"2560x1440": "qhd-1440p",
	"2048x1080": "2k-1080p",
	"3840x2160": "4k-2160p",
	"7680x4320": "8k-4320p",
}

var dimensionsRegexp = regexp.MustCompile(`\d+x\d+$`)

var insertDimensionsBefore = func(c *cli.Context, args []string, fi os.FileInfo, dryRun, verbose bool) error {
	fp := strings.Replace(fi.Name(), " ", "\\ ", -1)
	fp = strings.Replace(fp, "'", "\\'", -1)
	cmd := fmt.Sprintf(`ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=s=x:p=0 %s`, fp)

	output, err := exec(cmd)
	if err != nil {
		return fmt.Errorf("failed to probe file. command: '%s', err: %w", cmd, err)
	}

	output = strings.TrimSpace(output)
	if verbose {
		log.Printf("dimenensions found. file: '%s', dimensions: '%s'", fp, output)
	}

	output = dimensionsRegexp.FindString(output)
	if verbose {
		log.Printf("dimensions found in multiline output. file: '%s', dimensions: '%s'", fp, output)
	}

	if output == "" {
		return fmt.Errorf("failed to probe file, output was empty or invalid. command: '%s'", cmd)
	}

	if found, ok := wellKnown[output]; ok {
		output = found
	}

	return insertBefore(c, append(args, `-`+output), fi, dryRun, verbose)
}

func main() {
	app := &cli.App{
		Name: "ffr",
		Commands: []*cli.Command{
			{
				Name:        reencodeCommand,
				Usage:       reencodeUsage,
				ArgsUsage:   reencodeArgsUsage,
				Description: reencodeDescription,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
					&cli.BoolFlag{
						Name:    forceFlag,
						Aliases: []string{forceAlias},
						Value:   false,
						Usage:   forceUsage,
					},
					&cli.StringFlag{
						Name:  codecFlag,
						Usage: codecUsage,
						Value: defaultCodec,
					},
					&cli.StringFlag{
						Name:  presetFlag,
						Usage: fmt.Sprintf(presetUsage, strings.Join(allowedPresets, ", ")),
						Value: defaultPreset,
					},
					&cli.IntFlag{
						Name:  crfFlag,
						Usage: crfUsage,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, reEncode)
				},
			},
			{
				Name:      keyFramesCommand,
				Aliases:   strings.Split(keyFramesAliases, ", "),
				Usage:     keyFramesUsage,
				ArgsUsage: keyFramesArgsUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, keyFrames)
				},
			},
			{
				Name:      prefixCommand,
				Aliases:   strings.Split(prefixAliases, ", "),
				Usage:     prefixUsage,
				ArgsUsage: prefixArgsUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
					&cli.BoolFlag{
						Name:    forceFlag,
						Aliases: []string{forceAlias},
						Value:   false,
						Usage:   forceUsage,
					},
					&cli.IntFlag{
						Name:    skipPartsFlag,
						Aliases: []string{skipPartsAlias},
						Usage:   skipPartsUsage,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, prefix)
				},
			},
			{
				Name:      suffixCommand,
				Aliases:   strings.Split(suffixAliases, ", "),
				Usage:     suffixUsage,
				ArgsUsage: suffixArgsUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
					&cli.BoolFlag{
						Name:    forceFlag,
						Aliases: []string{forceAlias},
						Value:   false,
						Usage:   forceUsage,
					},
					&cli.IntFlag{
						Name:    skipPartsFlag,
						Aliases: []string{skipPartsAlias},
						Usage:   skipPartsUsage,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, suffix)
				},
			},
			{
				Name:      replaceCommand,
				Aliases:   strings.Split(replaceAliases, ", "),
				Usage:     replaceUsage,
				ArgsUsage: replaceArgsUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
					&cli.BoolFlag{
						Name:    forceFlag,
						Aliases: []string{forceAlias},
						Value:   false,
						Usage:   forceUsage,
					},
					&cli.IntFlag{
						Name:    skipFindsFlag,
						Aliases: []string{skipFindsAlias},
						Usage:   skipFindsUsage,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 2, replace)
				},
			},
			{
				Name:      mergePartsCommand,
				Aliases:   strings.Split(mergePartsAliases, ", "),
				Usage:     mergePartsUsage,
				ArgsUsage: mergePartsArgsUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
					&cli.BoolFlag{
						Name:    forceFlag,
						Aliases: []string{forceAlias},
						Value:   false,
						Usage:   forceUsage,
					},
					&cli.StringFlag{
						Name:    regexpFlag,
						Aliases: []string{regexpAlias},
						Value:   "",
						Usage:   regexpUsage,
					},
					&cli.StringFlag{
						Name:    deleteFlag,
						Aliases: []string{deleteAlias},
						Value:   "",
						Usage:   deleteUsage,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, mergeParts)
				},
			},
			{
				Name:      addNumberCommand,
				Aliases:   strings.Split(addNumberAliases, ", "),
				Usage:     addNumberUsage,
				ArgsUsage: addNumberArgsUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
					&cli.BoolFlag{
						Name:    forceFlag,
						Aliases: []string{forceAlias},
						Value:   false,
						Usage:   forceUsage,
					},
					&cli.StringFlag{
						Name:    regexpFlag,
						Aliases: []string{regexpAlias},
						Value:   "",
						Usage:   regexpUsage,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, addNumber)
				},
			},
			{
				Name:      deletePartsCommand,
				Aliases:   strings.Split(deletePartsAliases, ", "),
				Usage:     deletePartsUsage,
				ArgsUsage: deletePartsArgsUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
					&cli.BoolFlag{
						Name:    forceFlag,
						Aliases: []string{forceAlias},
						Value:   false,
						Usage:   forceUsage,
					},
					&cli.StringFlag{
						Name:    regexpFlag,
						Aliases: []string{regexpAlias},
						Value:   "",
						Usage:   regexpUsage,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, deleteParts)
				},
			},
			{
				Name:      insertBeforeCommand,
				Aliases:   strings.Split(insertBeforeAliases, ", "),
				Usage:     insertBeforeUsage,
				ArgsUsage: insertBeforeArgsUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
					&cli.BoolFlag{
						Name:    forceFlag,
						Aliases: []string{forceAlias},
						Value:   false,
						Usage:   forceUsage,
					},
					&cli.StringFlag{
						Name:    regexpFlag,
						Aliases: []string{regexpAlias},
						Value:   "",
						Usage:   regexpUsage,
					},
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, insertBefore)
				},
			},
			{
				Name:      insertDimensionsCommand,
				Aliases:   strings.Split(insertDimensionsAliases, ", "),
				Usage:     insertDimensionsUsage,
				ArgsUsage: insertDimensionsArgsUsage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    dryRunFlag,
						Aliases: []string{dryRunAlias},
						Value:   false,
						Usage:   dryRunUsage,
					},
					&cli.BoolFlag{
						Name:    verboseFlag,
						Aliases: []string{verboseAlias},
						Value:   false,
						Usage:   verboseUsage,
					},
					&cli.BoolFlag{
						Name:    forceFlag,
						Aliases: []string{forceAlias},
						Value:   false,
						Usage:   forceUsage,
					},
					&cli.StringFlag{
						Name:    regexpFlag,
						Aliases: []string{regexpAlias},
						Value:   "",
						Usage:   regexpUsage,
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

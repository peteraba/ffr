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
	"sync"
	"time"

	"github.com/bitfield/script"
	cli "github.com/urfave/cli/v2"
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

type logger struct {
	silent  bool
	history []string
}

func (l *logger) Printf(msg string, args ...interface{}) {
	if l.silent {
		l.history = append(l.history, fmt.Sprintf(msg, args...))
		return
	}

	log.Printf(msg, args...)
}

func (l *logger) Println(msg ...any) {
	if l.silent {
		l.history = append(l.history, fmt.Sprintln(msg...))
		return
	}

	log.Println(msg...)
}

var l logger

func safeRename(oldPath, newPath string, forceOverwrite bool) error {
	if oldPath == newPath {
		l.Printf("no file name change. path: '%s'", newPath)

		return nil
	}

	l.Println(oldPath, " -> ", newPath)

	_, err := os.Stat(newPath)
	if err == nil || !os.IsNotExist(err) {
		if !forceOverwrite {
			l.Printf("file already exists. path: '%s'", newPath)
			return err
		}

		l.Printf("force overwrite. path: '%s'", newPath)
	}

	err = os.Rename(oldPath, newPath)
	if err != nil {
		l.Printf("unexpected error during renaming file. old path: '%s', new path: '%s', err: %s", oldPath, newPath, err)
	}

	return err
}

func concat(parts []string, skip int, newPart, ext, separator string) string {
	if len(parts) < skip {
		panic(fmt.Errorf("unsafe usage of concat. len(parts): %d, skip: %d", len(parts), skip))
	}

	start := strings.Join(parts[:skip], separator)
	if start != "" {
		start += separator
	}

	end := strings.Join(parts[skip:], separator)
	if end != "" {
		end = separator + end
	}

	return start + newPart + end + ext
}

func getFileInfoList(filePaths []string, backwardsFlag bool) []os.FileInfo {
	if len(filePaths) == 0 {
		log.Fatalf("no files provided")

		return nil
	}

	var fileInfoList []os.FileInfo

	for _, filePath := range filePaths {
		fi, err := os.Stat(filePath)
		if err != nil {
			log.Fatalf("argument is not a file: %s, err: %s", filePath, err)
		}

		if fi.IsDir() {
			log.Fatalf("file is a directory: %s", filePath)
		}

		l.Printf("file is okay: %s", filePath)

		fileInfoList = append(fileInfoList, fi)
	}

	if backwardsFlag {
		var fis2 []os.FileInfo
		for i := len(fileInfoList) - 1; i >= 0; i-- {
			fis2 = append(fis2, fileInfoList[i])
		}
		fileInfoList = fis2
	}

	return fileInfoList
}

func process(c *cli.Context, argCount int, fn func(*cli.Context, []string, os.FileInfo, bool) error) error {
	args := c.Args().Slice()
	dryRun := c.Bool(dryRunFlag)

	l = logger{
		silent: !(c.Bool(verboseFlag) || c.Bool(dryRunFlag)),
	}

	if argCount > len(args) {
		return errors.New("not enough arguments")
	}

	fileInfoList := getFileInfoList(args[argCount:], c.Bool(backwardsFlag))
	for _, fi := range fileInfoList {
		l.Printf("file found: %s", fi.Name())
	}

	args = args[:argCount]

	t0 := time.Now()
	for _, fi := range fileInfoList {
		t1 := time.Now()
		err := fn(c, args, fi, dryRun)
		if err != nil {
			l.Println(err)
		}
		l.Printf("done in %s.", time.Since(t1).String())
	}
	l.Printf("all done in %s.", time.Since(t0).String())

	return nil
}

func exec(command string) (string, error) {
	p := script.Exec(command)
	output, err := p.String()
	if err != nil {
		l.Println(err)
	}

	return output, err
}

type App struct{}

func keyFrames(fi os.FileInfo) error {
	command := fmt.Sprintf(`ffprobe -loglevel error -select_streams v:0 -show_entries packet=pts_time,flags -of csv=print_section=0 "%s"`, fi.Name())

	l.Println(command)

	res, err := script.Exec(command).Match(",K__").FilterLine(func(line string) string {
		return strings.Split(line, ",")[0]
	}).Slice()

	if err != nil {
		return fmt.Errorf("unable to retrieve keyframes. err: %w", err)
	}

	maxCount := 4
	var numbers []string
	for i, line := range res {
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

	l.Printf("file: %s", fi.Name())
	l.Printf("indexes: %s...", strings.Join(numbers, ", "))

	return nil
}

func (a App) keyFrames(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	return keyFrames(fi)
}

const (
	videoCodecKey   = "-c:v"
	audioCodecKey   = "-c:a"
	crfKey          = "-crf"
	presetKey       = "-preset"
	audioQualityKey = "-q:a"
	losslessKey     = "-lossless"
)

type ReEncoder struct {
	lock     *sync.Mutex
	params   map[string]string
	order    []string
	keys     []string
	boolKeys []string
}

func NewReEncoder() *ReEncoder {
	return &ReEncoder{
		lock:     &sync.Mutex{},
		params:   make(map[string]string),
		keys:     []string{videoCodecKey, crfKey, losslessKey, presetKey},
		boolKeys: []string{losslessKey},
	}
}

func (r *ReEncoder) Set(key, value string) *ReEncoder {
	r.lock.Lock()
	defer r.lock.Unlock()

	_, ok := r.params[key]
	if ok {
		r.params[key] = value

		return r
	}

	r.params[key] = value
	r.order = append(r.order, key)

	return r
}

func (r *ReEncoder) Delete(key string) *ReEncoder {
	r.lock.Lock()
	defer r.lock.Unlock()

	_, ok := r.params[key]
	if !ok {
		return r
	}

	delete(r.params, key)
	for i, k := range r.order {
		if k == key {
			r.order = append(r.order[:i], r.order[i+1:]...)
		}
	}

	return r
}

func (r *ReEncoder) String() string {
	r.lock.Lock()
	defer r.lock.Unlock()

	params := []string{}
	for _, key := range r.order {
		params = append(params, key+" "+r.params[key])
	}

	return strings.Join(params, " ")
}

func (r *ReEncoder) GetPath() string {
	r.lock.Lock()
	defer r.lock.Unlock()

	values := []string{}

	for _, key := range r.keys {
		if value, ok := r.params[key]; ok {
			b := false
			for _, bv := range r.boolKeys {
				if bv == key {
					b = true
					break
				}
			}
			if b {
				values = append(values, strings.Trim(key, "-"))
			} else {
				values = append(values, value)
			}
		}
	}

	return strings.Join(values, "-")
}

func findPreset(preset string) (string, error) {
	for _, p := range allowedPresets {
		if p == preset {
			return preset, nil
		}
	}

	return "", fmt.Errorf("invalid preset. preset: %s", preset)
}

func reEncode(fi os.FileInfo, codec string, crf int, preset string, dryRun bool) (string, error) {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	extNew := "mp4"
	params := NewReEncoder()
	params.Set("-i", filePath).
		Set(crfKey, fmt.Sprintf("%d", crf)).
		Set(presetKey, preset)

	switch codec {
	case "libx265":
		// https://trac.ffmpeg.org/wiki/Encode/H.265
		if crf == 0 {
			crf = 28
		}

		preset, err := findPreset(preset)
		if err != nil {
			return "", err
		}

		params.
			Delete(crfKey).
			Set(videoCodecKey, "libx265").
			Set("-x265-params", "keyint=1").
			Set(presetKey, preset).
			Set(crfKey, fmt.Sprintf("%d", crf)).
			Set(audioCodecKey, "aac").
			Set(audioQualityKey, "100").
			Set("-tag:v", "hvc1")

		break
	case "libx264":
		// https://trac.ffmpeg.org/wiki/Encode/H.264
		if crf == 0 {
			crf = 23
		}

		preset, err := findPreset(preset)
		if err != nil {
			return "", err
		}

		params.
			Delete(crfKey).
			Set(videoCodecKey, "libx264").
			Set("-x264-params", "keyint=1").
			Set(presetKey, preset).
			Set(crfKey, fmt.Sprintf("%d", crf)).
			Set(audioCodecKey, "aac").
			Set(audioQualityKey, "100")

		break
	case "vp9":
		// https://trac.ffmpeg.org/wiki/Encode/VP9
		extNew = "mkv"

		params.
			Delete(presetKey).
			Delete(crfKey).
			Set(videoCodecKey, "vp9").
			Set("-g", "1").
			Set(crfKey, fmt.Sprintf("%d", crf)).
			Set("-b:v", "1").
			Set(audioCodecKey, "aac")

		if crf == 0 {
			params.Set(losslessKey, "1").Delete("-crf")
		}
	}

	outputPath := fmt.Sprintf("%s-%s.%s", basePath, params.GetPath(), extNew)
	command := fmt.Sprintf(`ffmpeg %s "%s"`, params.String(), outputPath)

	l.Printf("new path: %s", outputPath)
	l.Printf("command: %s", command)

	if dryRun {
		return outputPath, nil
	}

	output, err := exec(command)
	l.Println(output)

	return outputPath, err
}

func (a App) reEncode(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	codec := c.String(codecFlag)
	crf := c.Int(crfFlag)
	preset := c.String(presetFlag)

	_, err := reEncode(fi, codec, crf, preset, dryRun)

	return err
}

func prefix(fi os.FileInfo, newPart string, skip int, forceOverwrite bool, dryRun bool) error {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	parts := strings.Split(basePath, separator)

	newPath := concat(parts, skip, newPart, ext, separator)

	if dryRun {
		l.Println(filePath, " -> ", newPath)

		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) prefix(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	if len(args) == 0 {
		return nil
	}

	newPart := args[0]
	skip := c.Int(skipPartsFlag)
	forceOverwrite := c.Bool(forceFlag)

	return prefix(fi, newPart, skip, forceOverwrite, dryRun)
}

func suffix(fi os.FileInfo, newPart string, skip int, forceOverwrite, dryRun bool) error {
	filePath := fi.Name()

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

	if dryRun {
		l.Println(filePath, " -> ", newPath)

		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) suffix(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	skip := c.Int(skipPartsFlag)
	newPart := args[0]
	forceOverwrite := c.Bool(forceFlag)

	return suffix(fi, newPart, skip, forceOverwrite, dryRun)
}

func replace(fi os.FileInfo, search, replaceWith string, skip int, forceOverwrite bool, dryRun bool) error {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	parts := strings.Split(basePath, search)
	if skip > len(parts)-1 {
		return fmt.Errorf("more to skip than found occurances. file: '%s', skip: %d, found: %d", basePath, skip, len(parts)-1)
	}

	if len(parts) <= 1 {
		// safe rename is called to handle standard logging
		return safeRename(filePath, filePath, false)
	}

	start := strings.Join(parts[:skip+1], search)
	end := strings.Join(parts[skip+1:], search)

	newPath := start + replaceWith + end + ext
	l.Printf(`"%s" -> "%s", search: "%s", replace with: "%s"`, filePath, newPath, search, replaceWith)

	if dryRun {
		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) replace(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	if len(args) < 2 {
		return nil
	}

	search := args[0]
	replaceWith := args[1]
	skip := c.Int(skipFindsFlag)
	forceOverwrite := c.Bool(forceFlag)

	return replace(fi, search, replaceWith, skip, forceOverwrite, dryRun)
}

func mergeParts(fi os.FileInfo, regularExpression, deleteText string, forceOverwrite, dryRun bool) error {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

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

		l.Printf("base: %s", basePath)
		l.Printf("extra: %#v", extra)
		l.Printf("matches: %#v", m)
		l.Printf("sum: %d", sum)
		l.Println()
	}

	newPath := fmt.Sprintf("%s-%d%s%s", basePath, sum, strings.Join(extra, "-"), ext)
	if deleteText != "" {
		newPath = strings.Replace(newPath, deleteText, "", 1)
	}

	if dryRun {
		l.Printf(`"%s" -> "%s"`, filePath, newPath)

		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) mergeParts(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	regularExpression := c.String(regexpFlag)
	deleteText := c.String(deleteTextFlag)
	forceOverwrite := c.Bool(forceFlag)

	return mergeParts(fi, regularExpression, deleteText, forceOverwrite, dryRun)
}

func deleteRegexp(fi os.FileInfo, regularExpression string, regexpGroup, skipFinds, maxCount int, forceOverwrite, dryRun bool) error {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	if regularExpression == "" {
		regularExpression = `-\d+[a-z]+`
	}

	r, err := regexp.Compile(regularExpression)
	if err != nil {
		return err
	}

	matches := r.FindAllStringSubmatch(basePath, -1)
	l.Printf("basePath: %s", basePath)
	l.Printf("matches: %#v", matches)

	if len(matches) == 0 {
		return errors.New("no matches")
	}

	matches = matches[skipFinds:]
	for i, m := range matches {
		if maxCount > 0 && i >= maxCount {
			break
		}

		basePath = strings.Replace(basePath, m[regexpGroup], "", 1)
	}

	newPath := basePath + ext

	if dryRun {
		l.Printf(`"%s" -> "%s"`, filePath, newPath)

		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) deleteRegexp(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	regularExpression := c.String(regexpFlag)
	forceOverwrite := c.Bool(forceFlag)
	regexpGroup := c.Int(regexpGroupFlag)
	skipFinds := c.Int(skipFindsFlag)
	maxCount := c.Int(maxCountFlag)

	return deleteRegexp(fi, regularExpression, regexpGroup, skipFinds, maxCount, forceOverwrite, dryRun)
}

func deleteParts(fi os.FileInfo, partsToDelete []int, fromBack, forceOverwrite, dryRun bool) error {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	parts := strings.Split(basePath, "-")

	m := make(map[int]struct{}, len(partsToDelete))
	for _, p := range partsToDelete {
		p2 := p - 1
		if fromBack {
			p2 = len(parts) - p
		}
		m[p2] = struct{}{}
	}

	newParts := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		if _, ok := m[i]; !ok {
			newParts = append(newParts, parts[i])
		}
	}

	newPath := strings.Join(newParts, "-") + ext

	if dryRun {
		l.Printf(`"%s" -> "%s"`, filePath, newPath)

		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) deleteParts(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	forceOverwrite := c.Bool(forceFlag)
	fromBack := c.Bool(fromBackFlag)

	strList := strings.Split(args[0], ",")
	partsToDelete := make([]int, 0, len(strList))
	for _, str := range strList {
		num, err := strconv.ParseInt(str, 10, 32)
		if err != nil {
			panic(err)
		}

		partsToDelete = append(partsToDelete, int(num))
	}

	return deleteParts(fi, partsToDelete, fromBack, forceOverwrite, dryRun)
}

func addNumber(fi os.FileInfo, regularExpression string, numberToAdd int64, regexpGroup, skipFinds, maxCount int, forceOverwrite, dryRun bool) error {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	if regularExpression == "" {
		regularExpression = `-(\d+)[a-z]+`
		regexpGroup = 1
	}

	r, err := regexp.Compile(regularExpression)
	if err != nil {
		return err
	}

	matches := r.FindAllStringSubmatch(basePath, -1)
	l.Printf("basePath: %s", basePath)
	l.Printf("matches: %#v", matches)

	if len(matches) == 0 {
		return errors.New("no matches")
	}

	matches = matches[skipFinds:]
	for i, m := range matches {
		if maxCount > 0 && i >= maxCount {
			break
		}

		numberFound, err := strconv.ParseInt(m[regexpGroup], 10, 32)
		if err != nil {
			return err
		}

		n1 := strconv.Itoa(int(numberFound))
		n2 := strconv.Itoa(int(numberFound + numberToAdd))
		replaceWith := strings.Replace(m[0], n1, n2, 1)

		basePath = strings.Replace(basePath, m[0], replaceWith, 1)
	}

	newPath := basePath + ext

	if dryRun {
		l.Printf(`"%s" -> "%s"`, filePath, newPath)

		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) addNumber(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	regularExpression := c.String(regexpFlag)
	forceOverwrite := c.Bool(forceFlag)
	regexpGroup := c.Int(regexpGroupFlag)
	skipFinds := c.Int(skipFindsFlag)
	maxCount := c.Int(maxCountFlag)

	numberToAdd, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		return err
	}

	return addNumber(fi, regularExpression, numberToAdd, regexpGroup, skipFinds, maxCount, forceOverwrite, dryRun)
}

func insertBefore(fi os.FileInfo, regularExpression, insertText string, skipDashPrefix, forceOverwrite, dryRun bool) error {
	filePath := fi.Name()

	if regularExpression == "" {
		regularExpression = "\\d+[a-z]+"
	}

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	regularExpression = "(" + regularExpression + ")"
	if !skipDashPrefix {
		regularExpression = "-" + regularExpression
	}
	r, err := regexp.Compile(regularExpression)
	if err != nil {
		return fmt.Errorf("regexp failed, err: %w", err)
	}
	matched := r.FindAllStringSubmatch(basePath, -1)

	// fallback in case of no match is to insert text at the end of the string
	newPath := basePath + "-" + insertText + ext
	if len(matched) > 0 {
		insertText += "-" + matched[len(matched)-1][1]
		newPath = strings.Replace(basePath, matched[len(matched)-1][1], insertText, 1) + ext
	}

	l.Printf(`"%s" -> "%s", found: "%s", new: "%s"`, filePath, newPath, matched, insertText)

	if dryRun {
		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) insertBefore(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	regularExpression := c.String(regexpFlag)
	skipDashPrefix := c.Bool(skipDashPrefixFlag)
	insert := args[1]

	forceOverwrite := c.Bool(forceFlag)

	return insertBefore(fi, regularExpression, insert, skipDashPrefix, forceOverwrite, dryRun)
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

func getDimensions(fi os.FileInfo) (string, error) {
	fp := strings.Replace(fi.Name(), " ", "\\ ", -1)
	fp = strings.Replace(fp, "'", "\\'", -1)
	cmd := fmt.Sprintf(`ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=s=x:p=0 %s`, fp)
	l.Printf(cmd)

	dimensions, err := exec(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to probe file. command: '%s', err: %w", cmd, err)
	}

	if dimensions == "" {
		return "", fmt.Errorf("failed to probe file, output was empty or invalid. command: '%s'", cmd)
	}

	dimensions = strings.TrimSpace(dimensions)
	l.Printf("dimensions found. file: '%s', dimensions: '%s'", fp, dimensions)

	dimensions = dimensionsRegexp.FindString(dimensions)
	l.Printf("dimensions found in multiline output. file: '%s', dimensions: '%s'", fp, dimensions)

	if dimensions == "" {
		return "", fmt.Errorf("failed to probe file, output was empty or invalid. command: '%s'", cmd)
	}

	return dimensions, nil
}

func insertDimensionsBefore(fi os.FileInfo, regularExpression string, skipDashPrefix, forceOverwrite, dryRun bool) error {
	dimensions, err := getDimensions(fi)
	if err != nil {
		return err
	}

	if found, ok := wellKnown[dimensions]; ok {
		dimensions = found
	}

	return insertBefore(fi, regularExpression, dimensions, skipDashPrefix, forceOverwrite, dryRun)
}

func (a App) insertDimensionsBefore(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	regularExpression := c.String(regexpFlag)
	skipDashPrefix := c.Bool(skipDashPrefixFlag)
	forceOverwrite := c.Bool(forceFlag)

	return insertDimensionsBefore(fi, regularExpression, skipDashPrefix, forceOverwrite, dryRun)
}

func crop(fi os.FileInfo, width, height int, x, y string, forceOverwrite, dryRun bool) error {
	basePath := filepath.Base(fi.Name())
	ext := filepath.Ext(fi.Name())
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	dimensions, err := getDimensions(fi)
	if err != nil {
		return fmt.Errorf("failed to retrieve video dimensions. err: %w", err)
	}

	d := strings.Split(dimensions, "x")
	if len(d) != 2 {
		return fmt.Errorf("wrong old dimensions: %s", dimensions)
	}

	widthOrigin, err := strconv.Atoi(d[0])
	if err != nil {
		return fmt.Errorf("wrong old dimensions: %s", dimensions)
	}

	heightOrigin, err := strconv.Atoi(d[1])
	if err != nil {
		return fmt.Errorf("wrong old dimensions: %s", dimensions)
	}

	if widthOrigin < width || heightOrigin < height {
		return fmt.Errorf("wrong dimensions. new dimensions: %dx%d, old dimensions: %s", width, height, dimensions)
	}

	var xPos int
	switch x {
	case "left":
	case "center":
		xPos = (widthOrigin - width) / 2
	case "right":
		xPos = widthOrigin - width
	default:
		xPos, err = strconv.Atoi(x)
		if err != nil {
			return fmt.Errorf("wrong instructions, x: %s", x)
		}
	}

	var yPos int
	switch y {
	case "top":
	case "center":
		yPos = (heightOrigin - height) / 2
	case "bottom":
		yPos = heightOrigin - height
	default:
		yPos, err = strconv.Atoi(y)
		if err != nil {
			return fmt.Errorf("wrong instructions, y: %s", y)
		}
	}

	if widthOrigin < width+yPos || heightOrigin < height+xPos {
		return fmt.Errorf("wrong instructions. new dimensions: %dx%d, pox x: %d, pos y: %d, old dimensions: %s", width, height, xPos, yPos, dimensions)
	}

	newPath := fmt.Sprintf("%s-%dx%d%s", basePath, width, height, ext)

	cmd := fmt.Sprintf(`ffmpeg -i "%s" -filter:v "crop=%d:%d:%d:%d" "%s"`, fi.Name(), width, height, xPos, yPos, newPath)
	l.Printf(cmd)

	if dryRun {
		return nil
	}

	if !forceOverwrite {
		_, err = os.Stat(newPath)
		if err == nil || !os.IsNotExist(err) {
			return fmt.Errorf("file already exists. path: %s, err: %w", newPath, err)
		}
	}

	output, err := exec(cmd)
	if err != nil {
		l.Printf(output)

		return fmt.Errorf("failed to crop video. err: %w", err)
	}

	return nil
}

func (a App) crop(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	forceOverwrite := c.Bool(forceFlag)

	width, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("wrong instructions, width: %s", args[0])
	}
	height, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("wrong instructions, height: %s", args[1])
	}
	x := args[2]
	y := args[3]

	return crop(fi, width, height, x, y, forceOverwrite, dryRun)
}

// commands
const (
	addNumberCommand = "add-number"
	addNumberAliases = "a"
	addNumberUsage   = `add a number to the last number found in the file

EXAMPLES:
Description: Increment the last number segment in the file name 'foo-1080p-2ffc.mp4'
Command:     ffr add-number 2 foo-1080p-2ffc.mp4
Result:      foo-1080p-4ffc.mp4

Description: Increment the number in '1080p' in the file name 'foo-1080p-2ffc.mp4'
Command:     ffr add-number --regular-expression '-(\d+)p' 2 foo-1080p-2ffc.mp4
Result:      foo-1080p-4ffc.mp4`
	addNumberArgsUsage = "[number-to-addNumber] [files...]"

	deletePartsCommand   = "delete-parts"
	deletePartsAliases   = "dp"
	deletePartsUsage     = "delete certain parts based on a comma separated list of parts"
	deletePartsArgsUsage = `[comma-separated-list] [files...]

EXAMPLES:
Description: Delete the first and third segments in the file name 'foo-bar-baz-2ffc.mp4'
Command:     ffr delete-parts 1,3 foo-bar-baz-2ffc.mp4
Result:      bar-2ffc.mp4

Description: Delete the last and the third last segments in the file name 'foo-bar-baz-2ffc.mp4'
Command:     ffr delete-parts --fb 1,3 foo-bar-baz-2ffc.mp4
Result:      foo-baz.mp4`

	deleteRegexpCommand   = "delete-regexp"
	deleteRegexpAliases   = "dr"
	deleteRegexpUsage     = "delete a part based on regular expression"
	deleteRegexpArgsUsage = "[files...]"

	insertBeforeCommand   = "insert-before"
	insertBeforeAliases   = "ib"
	insertBeforeUsage     = "insert before the generated descriptions"
	insertBeforeArgsUsage = "[text to insert] [files...]"

	insertDimensionsCommand   = "insert-dimensions"
	insertDimensionsAliases   = "id"
	insertDimensionsUsage     = "insert video dimensions before the generated descriptions"
	insertDimensionsArgsUsage = "[files...]"

	keyFramesCommand   = "keyframes"
	keyFramesAliases   = "k"
	keyFramesUsage     = "list keyframes of video file(s)"
	keyFramesArgsUsage = "[files...]"

	mergePartsCommand   = "merge-parts"
	mergePartsAliases   = "m"
	mergePartsUsage     = "merge the generated descriptions [foo-12ffc-1bar -> abc-12bar]"
	mergePartsArgsUsage = "[files...]"

	prefixCommand   = "prefix"
	prefixAliases   = "p"
	prefixUsage     = "prefix file names with a fixed string"
	prefixArgsUsage = "[text to insert] [files...]"

	reencodeCommand     = "reencode"
	reencodeUsage       = "reencode a file via ffmpeg"
	reencodeArgsUsage   = "[files...]"
	reencodeDescription = `
Find more about the various codecs and their settings here:
https://trac.ffmpeg.org/wiki/Encode/H.265
https://trac.ffmpeg.org/wiki/Encode/H.264
https://trac.ffmpeg.org/wiki/Encode/VP9`

	replaceCommand   = "replace"
	replaceAliases   = "r"
	replaceUsage     = "replace a fixed string in file names"
	replaceArgsUsage = "[needle] [text to insert] [files...]"

	suffixCommand   = "suffix"
	suffixAliases   = "s"
	suffixUsage     = "suffix file names with a fixed string"
	suffixArgsUsage = "[text to insert] [files...]"

	cropCommand   = "crop"
	cropAliases   = "c"
	cropUsage     = "crop video"
	cropArgsUsage = "[width] [height] [left|center|right|px from left] [top|center|bottom|px from top] [files...]"
)

// flags
const (
	backwardsFlag  = "backwards"
	backwardsAlias = "b"
	backwardsUsage = "loop over the files backwards"

	deleteTextFlag  = "delete-text"
	deleteTextAlias = "del"
	deleteTextUsage = "text to delete after merging"

	dryRunFlag  = "dryRun"
	dryRunAlias = "d"
	dryRunUsage = "only print commands, do not execute anything"

	codecFlag  = "codec"
	codecUsage = "codec to use for encoding [libx264, libx265, vp9]"

	crfFlag  = "crf"
	crfUsage = "crf to use for encoding [https://slhck.info/video/2017/02/24/crf-guide.html]"

	forceFlag  = "force-overwrite"
	forceAlias = "f"
	forceUsage = "force overwriting existing files"

	fromBackFlag  = "from-back"
	fromBackAlias = "fb"
	fromBackUsage = "comma separated list of part counts to change"

	maxCountFlag  = "max-count"
	maxCountAlias = "mc"
	maxCountUsage = "maximum count of changes. 0 means no maximum."

	partsFlag  = "parts"
	partsAlias = "p"
	partsUsage = "comma separated list of part counts to change"

	presetFlag  = "preset"
	presetUsage = "preset to use for encoding [%s] (x264, x265 only)"

	skipFindsFlag  = "skip-finds"
	skipFindsAlias = "s"
	skipFindsUsage = "number finds to skip"

	skipPartsFlag  = "skip-parts"
	skipPartsAlias = "s"
	skipPartsUsage = "number of dash-separated parts to skip"

	regexpFlag  = "regular-expression"
	regexpAlias = "r"
	regexpUsage = "regular expression which could be used to filter parts"

	regexpGroupFlag  = "regexp-group"
	regexpGroupAlias = "rg"
	regexpGroupUsage = "regexp group number to use"

	skipDashPrefixFlag  = "skip-dash-prefix"
	skipDashPrefixAlias = "sdp"
	skipDashPrefixUsage = "if true, the regular expression will not be prefixed with a dash"

	verboseFlag  = "verbose"
	verboseAlias = "v"
	verboseUsage = "print commands before executing them"
)

func main() {
	a := App{}

	allFlags := map[string]cli.Flag{
		backwardsFlag: &cli.BoolFlag{
			Name:    backwardsFlag,
			Aliases: []string{backwardsAlias},
			Value:   true,
			Usage:   backwardsUsage,
		},
		dryRunFlag: &cli.BoolFlag{
			Name:    dryRunFlag,
			Aliases: []string{dryRunAlias},
			Value:   false,
			Usage:   dryRunUsage,
		},
		verboseFlag: &cli.BoolFlag{
			Name:    verboseFlag,
			Aliases: []string{verboseAlias},
			Value:   false,
			Usage:   verboseUsage,
		},
		forceFlag: &cli.BoolFlag{
			Name:    forceFlag,
			Aliases: []string{forceAlias},
			Value:   false,
			Usage:   forceUsage,
		},
		codecFlag: &cli.StringFlag{
			Name:  codecFlag,
			Usage: codecUsage,
			Value: defaultCodec,
		},
		presetFlag: &cli.StringFlag{
			Name:  presetFlag,
			Usage: fmt.Sprintf(presetUsage, strings.Join(allowedPresets, ", ")),
			Value: defaultPreset,
		},
		crfFlag: &cli.IntFlag{
			Name:  crfFlag,
			Usage: crfUsage,
		},
		skipPartsFlag: &cli.IntFlag{
			Name:    skipPartsFlag,
			Aliases: []string{skipPartsAlias},
			Usage:   skipPartsUsage,
		},
		skipFindsFlag: &cli.IntFlag{
			Name:    skipFindsFlag,
			Aliases: []string{skipFindsAlias},
			Usage:   skipFindsUsage,
		},
		regexpFlag: &cli.StringFlag{
			Name:    regexpFlag,
			Aliases: []string{regexpAlias},
			Value:   "",
			Usage:   regexpUsage,
		},
		skipDashPrefixFlag: &cli.BoolFlag{
			Name:    skipDashPrefixFlag,
			Aliases: []string{skipDashPrefixAlias},
			Value:   true,
			Usage:   skipDashPrefixUsage,
		},
		deleteTextFlag: &cli.StringFlag{
			Name:    deleteTextFlag,
			Aliases: []string{deleteTextAlias},
			Value:   "",
			Usage:   deleteTextUsage,
		},
		regexpGroupFlag: &cli.IntFlag{
			Name:    regexpGroupFlag,
			Aliases: []string{regexpGroupAlias},
			Value:   0,
			Usage:   regexpGroupUsage,
		},
		maxCountFlag: &cli.IntFlag{
			Name:    maxCountFlag,
			Aliases: []string{maxCountAlias},
			Value:   1,
			Usage:   maxCountUsage,
		},
		partsFlag: &cli.StringFlag{
			Name:    partsFlag,
			Aliases: []string{partsAlias},
			Value:   "",
			Usage:   partsUsage,
		},
		fromBackFlag: &cli.BoolFlag{
			Name:    fromBackFlag,
			Aliases: []string{fromBackAlias},
			Value:   false,
			Usage:   fromBackUsage,
		},
	}

	app := &cli.App{
		Name: "ffr",
		Commands: []*cli.Command{
			{
				Name:      addNumberCommand,
				Aliases:   strings.Split(addNumberAliases, ", "),
				Usage:     addNumberUsage,
				ArgsUsage: addNumberArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[maxCountFlag],
					allFlags[regexpFlag],
					allFlags[regexpGroupFlag],
					allFlags[skipFindsFlag],
					allFlags[verboseFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, a.addNumber)
				},
			},
			{
				Name:      deletePartsCommand,
				Aliases:   strings.Split(deletePartsAliases, ", "),
				Usage:     deletePartsUsage,
				ArgsUsage: deletePartsArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[fromBackFlag],
					allFlags[partsFlag],
					allFlags[verboseFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, a.deleteParts)
				},
			},
			{
				Name:      deleteRegexpCommand,
				Aliases:   strings.Split(deleteRegexpAliases, ", "),
				Usage:     deleteRegexpUsage,
				ArgsUsage: deleteRegexpArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[maxCountFlag],
					allFlags[regexpFlag],
					allFlags[regexpGroupFlag],
					allFlags[verboseFlag],
					allFlags[skipPartsFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, a.deleteRegexp)
				},
			},
			{
				Name:      insertBeforeCommand,
				Aliases:   strings.Split(insertBeforeAliases, ", "),
				Usage:     insertBeforeUsage,
				ArgsUsage: insertBeforeArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[regexpFlag],
					allFlags[verboseFlag],
					allFlags[skipDashPrefixFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, a.insertBefore)
				},
			},
			{
				Name:      insertDimensionsCommand,
				Aliases:   strings.Split(insertDimensionsAliases, ", "),
				Usage:     insertDimensionsUsage,
				ArgsUsage: insertDimensionsArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[regexpFlag],
					allFlags[verboseFlag],
					allFlags[skipDashPrefixFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, a.insertDimensionsBefore)
				},
			},
			{
				Name:      keyFramesCommand,
				Aliases:   strings.Split(keyFramesAliases, ", "),
				Usage:     keyFramesUsage,
				ArgsUsage: keyFramesArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[verboseFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, a.keyFrames)
				},
			},
			{
				Name:      mergePartsCommand,
				Aliases:   strings.Split(mergePartsAliases, ", "),
				Usage:     mergePartsUsage,
				ArgsUsage: mergePartsArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[deleteTextFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[regexpFlag],
					allFlags[skipPartsFlag],
					allFlags[verboseFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, a.mergeParts)
				},
			},
			{
				Name:      prefixCommand,
				Aliases:   strings.Split(prefixAliases, ", "),
				Usage:     prefixUsage,
				ArgsUsage: prefixArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[skipPartsFlag],
					allFlags[verboseFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, a.prefix)
				},
			},
			{
				Name:        reencodeCommand,
				Usage:       reencodeUsage,
				ArgsUsage:   reencodeArgsUsage,
				Description: reencodeDescription,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[codecFlag],
					allFlags[crfFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[presetFlag],
					allFlags[verboseFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, a.reEncode)
				},
			},
			{
				Name:      replaceCommand,
				Aliases:   strings.Split(replaceAliases, ", "),
				Usage:     replaceUsage,
				ArgsUsage: replaceArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[skipFindsFlag],
					allFlags[verboseFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 2, a.replace)
				},
			},
			{
				Name:      suffixCommand,
				Aliases:   strings.Split(suffixAliases, ", "),
				Usage:     suffixUsage,
				ArgsUsage: suffixArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[skipPartsFlag],
					allFlags[verboseFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 1, a.suffix)
				},
			},
			{
				Name:      cropCommand,
				Aliases:   strings.Split(cropAliases, ", "),
				Usage:     cropUsage,
				ArgsUsage: cropArgsUsage,
				Flags: []cli.Flag{
					allFlags[backwardsFlag],
					allFlags[dryRunFlag],
					allFlags[forceFlag],
					allFlags[verboseFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 4, a.crop)
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

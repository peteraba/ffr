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
	"github.com/cheynewallace/tabby"
	cli "github.com/urfave/cli/v2"
)

const (
	separator = "-"
)

const (
	codecH264 = "h264"
	codecH265 = "hevc"
)

const (
	encoderH264 = "libx264"
	encoderH265 = "libx265"
	encoderVP9  = "vp9"
)

const (
	eightKPreset  = "8k"
	fourKPreset   = "4k"
	qHDPreset     = "qhd"
	twoKPreset    = "2k"
	fullHDPreset  = "fullhd"
	hdPreset      = "hd"
	sdPreset      = "sd"
	eightKPreset2 = "4320p"
	fourKPreset2  = "2160p"
	qHDPreset2    = "1440p"
	fullHDPreset2 = "1080p"
	hdPreset2     = "720p"
	edPreset      = "540p"
	sdPreset2     = "480p"
)

const (
	eightKWidth  = 7680
	eightKHeight = 4320
	fourKWidth   = 3840
	fourKHeight  = 2160
	qHDWidth     = 2560
	qHDHeight    = 1440
	twoKWidth    = 2048
	twoKHeight   = 1080
	fullHDWidth  = 1920
	fullHDHeight = 1080
	hdWidth      = 1280
	hdHeight     = 720
	edWidth      = 960
	edHeight     = 540
	sdWidth      = 640
	sdHeight     = 480
)

var wellKnown = map[string]string{
	"7680x4320": "8k-4320p",
	"3840x2160": "4k-2160p",
	"2048x1080": "2k-1080p",
	"2560x1440": "qhd-1440p",
	"1920x1080": "fullhd-1080p",
	"1280x720":  "hd-720p",
	"960x540":   "ed-540p",
	"640x480":   "sd-480p",
}

const (
	defaultCodec  = encoderH265
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
			l.Printf("file already exists. path: %q", newPath)
			return err
		}

		l.Printf("force overwrite. path: %q", newPath)
	}

	err = os.Rename(oldPath, newPath)
	if err != nil {
		l.Printf("unexpected error during renaming file. old path: %q, new path: %q, err: %s", oldPath, newPath, err)
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
			log.Fatalf("argument is not a file: %q, err: %s", filePath, err)
		}

		if fi.IsDir() {
			log.Fatalf("file is a directory: %q", filePath)
		}

		l.Printf("file is okay: %q", filePath)

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
		l.Printf("file found: %q", fi.Name())
	}

	args = args[:argCount]

	t0 := time.Now()
	for _, fi := range fileInfoList {
		t1 := time.Now()
		err := fn(c, args, fi, dryRun)
		if err != nil {
			l.Println(err)
		}
		log.Printf("done in %s.", time.Since(t1).String())
	}
	log.Printf("all done in %s.", time.Since(t0).String())

	return nil
}

func processAll(c *cli.Context, argCount int, fn func(*cli.Context, []string, []os.FileInfo, bool) error) error {
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
		l.Printf("file found: %q", fi.Name())
	}

	args = args[:argCount]

	t0 := time.Now()
	err := fn(c, args, fileInfoList, dryRun)
	if err != nil {
		l.Println(err)
	}
	log.Printf("all done in %s.", time.Since(t0).String())

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

func findKeyFrames(fi os.FileInfo) ([]string, error) {
	command := fmt.Sprintf(`ffprobe -loglevel error -select_streams v:0 -show_entries packet=pts_time,flags -of csv=print_section=0 %q`, fi.Name())

	res, err := script.Exec(command).Match(",K__").FilterLine(func(line string) string {
		return strings.Split(line, ",")[0]
	}).Slice()

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve keyframes. err: %w", err)
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
			return nil, err
		}

		numbers = append(numbers, fmt.Sprintf("%.1f", n))
	}

	return numbers, nil
}

func keyFrames(fi os.FileInfo) error {
	numbers, err := findKeyFrames(fi)
	if err != nil {
		return err
	}

	l.Printf("file: %s", fi.Name())
	l.Printf("indexes: %s...", strings.Join(numbers, ", "))

	return nil
}

func (a App) keyFrames(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	return keyFrames(fi)
}

const (
	videoCodecKey    = "-c:v"
	audioCodecKey    = "-c:a"
	crfKey           = "-crf"
	bitRateKey       = "-b:v"
	maxRateKey       = "-maxrate"
	bufsizeKey       = "-bufsize"
	presetKey        = "-preset"
	losslessKey      = "-lossless"
	hwaccelKey       = "-hwaccel"
	hwaccelDeviceKey = "-hwaccel_device"
	inputKey         = "-i"
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
		keys:     []string{videoCodecKey, hwaccelKey, crfKey, losslessKey, presetKey},
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
		params = append(params, fmt.Sprintf("%s %q", key, r.params[key]))
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

func getNewBitRates(fi os.FileInfo, encoder string) (string, string, error) {
	oldCodec, err := getCodec(fi)
	if err != nil {
		return "", "", fmt.Errorf("unable to get codec. err: %w", err)
	}

	rawBitRate, err := getBitRate(fi)
	if err != nil {
		return "", "", fmt.Errorf("unable to get bitrate. err: %w", err)
	}

	if rawBitRate == 0 {
		vt := info(fi, true)

		rawBitRate = vt.width * vt.height / 10 * int64(vt.frameRate)
	}

	rbr := intToString(rawBitRate, "", "")
	l.Printf("file: %s, old codec: %s, encoder: %s, old bit rate: %d, rbr human: %s", fi.Name(), oldCodec, encoder, rawBitRate, rbr)

	if encoder == encoderH265 && oldCodec != codecH265 {
		rawBitRate = rawBitRate * 6 / 10
	}

	rbr = intToString(rawBitRate, "", "")
	rbr2 := intToString(rawBitRate*2, "", "")
	l.Printf("file: %s, old codec: %s, encoder: %s, new bit rate: %d, rbr human: %s", fi.Name(), oldCodec, encoder, rawBitRate, rbr)

	return rbr, rbr2, nil
}

func reEncode(fi os.FileInfo, codec string, crf int, preset, hwaccel, hwaccelDevice string, replaceFile, dryRun bool) (string, error) {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	extNew := "mp4"
	params := NewReEncoder()
	params.
		Set(hwaccelKey, "auto").
		Set(hwaccelDeviceKey, hwaccelDevice).
		Set(inputKey, filePath).
		Set(crfKey, fmt.Sprintf("%d", crf)).
		Set(presetKey, preset)

	switch codec {
	case encoderH265:
		const x265Params = "-x265-params"

		// https://trac.ffmpeg.org/wiki/Encode/H.265
		if crf == 0 {
			crf = 23
		}

		preset, err := findPreset(preset)
		if err != nil {
			return "", err
		}

		params.
			Delete(crfKey).
			Set(videoCodecKey, encoderH265).
			Set(x265Params, "keyint=1").
			Set(presetKey, preset).
			Set(crfKey, fmt.Sprintf("%d", crf)).
			Set(audioCodecKey, "copy").
			Set("-tag:v", "hvc1")

		switch hwaccel {
		case "qsv":
			params.
				Delete(presetKey).
				Delete(crfKey).
				// Set(hwaccelKey, "hevc_qsv").
				Set(videoCodecKey, "hevc_qsv")
		default:
			params.
				Delete(hwaccelKey).
				Delete(hwaccelDeviceKey)
		}

		break
	case encoderH264:
		const x264Params = "-x264-params"

		// https://trac.ffmpeg.org/wiki/Encode/H.264
		if crf == 0 {
			crf = 20
		}

		preset, err := findPreset(preset)
		if err != nil {
			return "", err
		}

		params.
			Delete(crfKey).
			Set(videoCodecKey, encoderH264).
			Set(x264Params, "keyint=1").
			Set(presetKey, preset).
			Set(crfKey, fmt.Sprintf("%d", crf)).
			Set(audioCodecKey, "copy")

		switch hwaccel {
		case "qsv":
			params.
				Delete(presetKey).
				Delete(crfKey).
				// Set(hwaccelKey, "hevc_qsv").
				Set(videoCodecKey, "h264_qsv")
		default:
			params.
				Delete(hwaccelKey).
				Delete(hwaccelDeviceKey)
		}

		break
	case encoderVP9:
		const vp9KeyFrameKey = "-g"

		// https://trac.ffmpeg.org/wiki/Encode/VP9
		extNew = "mkv"

		params.
			Delete(presetKey).
			Delete(crfKey).
			Set(videoCodecKey, encoderVP9).
			Set(vp9KeyFrameKey, "1").
			Set(crfKey, fmt.Sprintf("%d", crf)).
			Set(audioCodecKey, "copy")

		if crf == 0 {
			params.
				Delete(crfKey).
				Set(losslessKey, "1")
		}

		switch hwaccel {
		case "qsv":
			params.
				Delete(presetKey).
				Delete(crfKey).
				// Set(hwaccelKey, "hevc_qsv").
				Set(videoCodecKey, "vp9_qsv")
		default:
			params.
				Delete(hwaccelKey).
				Delete(hwaccelDeviceKey)
		}
	}

	if hwaccel != "" {
		avgBitRate, maxBitRate, err := getNewBitRates(fi, codec)
		if err != nil {
			return "", fmt.Errorf("unable to get bit rates. err: %w", err)
		}

		params.
			Set(bitRateKey, avgBitRate).
			Set(maxRateKey, maxBitRate).
			Set(bufsizeKey, maxBitRate)
	}

	outputPath := fmt.Sprintf("%s-%s.%s", basePath, params.GetPath(), extNew)
	i := 1
	for {
		_, err := os.Stat(outputPath)
		if err != nil {
			break
		}

		l.Printf("file exists: %s", outputPath)

		outputPath = fmt.Sprintf("%s-%s%d.%s", basePath, params.GetPath(), i, extNew)
		i++
	}

	command := fmt.Sprintf(`ffmpeg %s %q`, params.String(), outputPath)

	l.Printf("new path: %s", outputPath)
	l.Printf("command: %s", command)

	if dryRun {
		return outputPath, nil
	}

	output, err := exec(command)
	l.Println(output)

	if replaceFile {
		backupFile := fmt.Sprintf("%s-backup.%s", basePath, extNew)

		l.Printf(fmt.Sprintf("mv %s %s", filePath, backupFile))
		l.Printf(fmt.Sprintf("mv %s %s", outputPath, filePath))

		exec(fmt.Sprintf("mv %s %s", filePath, backupFile))
		exec(fmt.Sprintf("mv %s %s", outputPath, filePath))
	}

	return outputPath, err
}

func (a App) reEncode(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	codec := c.String(codecFlag)
	crf := c.Int(crfFlag)
	preset := c.String(presetFlag)
	hwaccel := c.String(hwaccelFlag)
	hwaccelDevice := c.String(hwaccelDeviceFlag)
	replaceFile := c.Bool(replaceFileFlag)

	_, err := reEncode(fi, codec, crf, preset, hwaccel, hwaccelDevice, replaceFile, dryRun)

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
		return fmt.Errorf("more to skip then parts present. file: %q skip: %d, parts: %d", basePath, skip, len(parts))
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
		return fmt.Errorf("more to skip than found occurances. file: %q, skip: %d, found: %d", basePath, skip, len(parts)-1)
	}

	if len(parts) <= 1 {
		// safe rename is called to handle standard logging
		return safeRename(filePath, filePath, false)
	}

	start := strings.Join(parts[:skip+1], search)
	end := strings.Join(parts[skip+1:], search)

	newPath := start + replaceWith + end + ext
	l.Printf(`%q -> %q, search: %q, replace with: %q`, filePath, newPath, search, replaceWith)

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

	r, err := regexp.Compile(`-(\d{1,2})(` + regularExpression + `(-[a-z]+\d*)*)`)
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
		l.Printf(`%q -> %q`, filePath, newPath)

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
		l.Printf(`%q -> %q`, filePath, newPath)

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
		l.Printf(`%q -> %q`, filePath, newPath)

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
		l.Printf(`%q -> %q`, filePath, newPath)

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

func insertBefore(fi os.FileInfo, regularExpression, insertText string, skipDuplicate, skipDashPrefix, forceOverwrite, dryRun bool) error {
	filePath := fi.Name()

	if regularExpression == "" {
		regularExpression = "\\d+[a-z]+"
	}

	if skipDuplicate && strings.Contains(filePath, insertText) {
		l.Printf(`skipping as duplicate is found. needle: %q, haystack: %q`, insertText, filePath)

		return nil
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

	l.Printf(`%q -> %q, found: %q, new: %q`, filePath, newPath, matched, insertText)

	if dryRun {
		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) insertBefore(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	regularExpression := c.String(regexpFlag)
	skipDashPrefix := c.Bool(skipDashPrefixFlag)
	skipDuplicate := c.Bool(skipDuplicateFlag)
	insert := args[1]

	forceOverwrite := c.Bool(forceFlag)

	return insertBefore(fi, regularExpression, insert, skipDuplicate, skipDashPrefix, forceOverwrite, dryRun)
}

var dimensionsRegexp = regexp.MustCompile(`\d+x\d+$`)

func getDimensions(fi os.FileInfo) (string, error) {
	fp := strings.Replace(fi.Name(), " ", "\\ ", -1)
	fp = strings.Replace(fp, "'", "\\'", -1)
	cmd := fmt.Sprintf(`ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=s=x:p=0 %s`, fp)

	dimensions, err := exec(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to probe file. command: %q, err: %w", cmd, err)
	}

	if dimensions == "" {
		return "", fmt.Errorf("failed to probe file, output was empty or invalid. command: %q", cmd)
	}

	dimensions = strings.TrimSpace(dimensions)

	dimensions = dimensionsRegexp.FindString(dimensions)

	if dimensions == "" {
		return "", fmt.Errorf("failed to probe file, output was empty or invalid. command: %q", cmd)
	}

	return dimensions, nil
}

func insertDimensionsBefore(fi os.FileInfo, regularExpression string, skipDuplicatePrefix, skipDashPrefix, forceOverwrite, dryRun bool) error {
	dimensions, err := getDimensions(fi)
	if err != nil {
		return err
	}

	if found, ok := wellKnown[dimensions]; ok {
		dimensions = found
	}

	return insertBefore(fi, regularExpression, dimensions, skipDuplicatePrefix, skipDashPrefix, forceOverwrite, dryRun)
}

var dateRegexp1 = regexp.MustCompile(`20\d{6}`)
var dateRegexp2 = regexp.MustCompile(`\d{6}`)
var dateFormat1 = "20060102"
var dateFormat2 = "060102"
var dateFormat3 = "2006.01.02"

func prefixDate(fi os.FileInfo, forceOverwrite, dryRun bool) error {
	filePath := fi.Name()

	basePath := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	matches := dateRegexp1.FindAllString(basePath, -1)
	format := dateFormat1
	l.Printf("basePath: %s", basePath)
	l.Printf("matches: %#v", matches)

	if len(matches) == 0 {
		matches = dateRegexp2.FindAllString(basePath, -1)
		format = dateFormat2
		l.Printf("basePath: %s", basePath)
		l.Printf("matches: %#v", matches)

		if len(matches) == 0 {
			return errors.New("no matches")
		}
	}

	if len(matches) > 1 {
		return errors.New("too many matches")
	}

	parsedDate, err := time.Parse(format, matches[0])
	if err != nil {
		return fmt.Errorf("failed to parse date. err: %w", err)
	}

	newPath := parsedDate.Format(dateFormat3) + "-" + basePath + ext

	if dryRun {
		l.Printf(`%q -> %q`, filePath, newPath)

		return nil
	}

	return safeRename(filePath, newPath, forceOverwrite)
}

func (a App) datePrefix(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	forceOverwrite := c.Bool(forceFlag)

	return prefixDate(fi, forceOverwrite, dryRun)
}

func (a App) insertDimensionsBefore(c *cli.Context, args []string, fi os.FileInfo, dryRun bool) error {
	regularExpression := c.String(regexpFlag)
	skipDashPrefix := c.Bool(skipDashPrefixFlag)
	skipDuplicatePrefix := c.Bool(skipDuplicateFlag)
	forceOverwrite := c.Bool(forceFlag)

	return insertDimensionsBefore(fi, regularExpression, skipDuplicatePrefix, skipDashPrefix, forceOverwrite, dryRun)
}

func parseDimensions(dimensions string) (int, int, error) {
	d := strings.Split(dimensions, "x")
	if len(d) != 2 {
		return 0, 0, fmt.Errorf("wrong old dimensions: %s", dimensions)
	}

	widthOrigin, err := strconv.Atoi(d[0])
	if err != nil {
		return 0, 0, fmt.Errorf("wrong old dimensions: %s", dimensions)
	}

	heightOrigin, err := strconv.Atoi(d[1])
	if err != nil {
		return 0, 0, fmt.Errorf("wrong old dimensions: %s", dimensions)
	}

	return widthOrigin, heightOrigin, nil
}

func crop(fi os.FileInfo, width, height int, x, y, dimensionPreset string, forceOverwrite, dryRun bool) error {
	basePath := filepath.Base(fi.Name())
	ext := filepath.Ext(fi.Name())
	if ext != "" {
		basePath = basePath[:len(basePath)-len(ext)]
	}

	switch dimensionPreset {
	case eightKPreset, eightKPreset2:
		width = eightKWidth
		height = eightKHeight
	case fourKPreset, fourKPreset2:
		width = fourKWidth
		height = fourKHeight
	case qHDPreset, qHDPreset2:
		width = qHDWidth
		height = qHDHeight
	case twoKPreset:
		width = twoKWidth
		height = twoKHeight
	case fullHDPreset, fullHDPreset2:
		width = fullHDWidth
		height = fullHDHeight
	case hdPreset, hdPreset2:
		width = hdWidth
		height = hdHeight
	case edPreset:
		width = edWidth
		height = edHeight
	case sdPreset, sdPreset2:
		width = sdWidth
		height = sdHeight
	}

	l.Printf("preset: %s, width: %d, height: %d", dimensionPreset, width, height)

	if width == 0 || height == 0 {
		return fmt.Errorf("wrong dimensions. width: %d, height: %d", width, height)
	}

	dimensions, err := getDimensions(fi)
	if err != nil {
		return fmt.Errorf("failed to retrieve video dimensions. err: %w", err)
	}

	widthOrigin, heightOrigin, err := parseDimensions(dimensions)
	if err != nil {
		return fmt.Errorf("failed to parse video dimensions. err: %w", err)
	}

	l.Printf("origin width: %d, origin height: %d", width, height)

	if widthOrigin < width || heightOrigin < height {
		return fmt.Errorf("wrong dimensions. new dimensions: %dx%d, old dimensions: %s", width, height, dimensions)
	}

	var xPos int
	switch x {
	case "left":
	case "center", "":
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
	case "center", "":
		yPos = (heightOrigin - height) / 2
	case "bottom":
		yPos = heightOrigin - height
	default:
		yPos, err = strconv.Atoi(y)
		if err != nil {
			return fmt.Errorf("wrong instructions, y: %s", y)
		}
	}

	l.Printf("x: %d, y: %d", xPos, yPos)

	if widthOrigin < width+yPos || heightOrigin < height+xPos {
		return fmt.Errorf("wrong instructions. new dimensions: %dx%d, pos x: %d, pos y: %d, old dimensions: %s", width, height, xPos, yPos, dimensions)
	}

	newPath := fmt.Sprintf("%s-%dx%d%s", basePath, width, height, ext)

	cmd := fmt.Sprintf(`ffmpeg -i %q -filter:v "crop=%d:%d:%d:%d" %q`, fi.Name(), width, height, xPos, yPos, newPath)
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

	width := c.Int(widthFlag)
	height := c.Int(heightFlag)
	x := c.String(xFlag)
	y := c.String(yFlag)

	dimensionPreset := c.String(dimensionPresetFlag)

	return crop(fi, width, height, x, y, dimensionPreset, forceOverwrite, dryRun)
}

type videoType struct {
	name      string
	size      int64
	bitRate   int64
	length    float64
	frameRate float64
	width     int64
	height    int64
	codec     string
	indexes   []string
}

type videoTypes []videoType

func (vs videoTypes) Print(skipKeyFrames bool, maxNameLength int) {
	t := tabby.New()
	t.AddHeader("FILE", "SIZE", "BITRATE", "LENGTH", "FRAMERATE", "WIDTH", "HEIGHT", "CODEC", "INDEXES")

	for _, v := range vs {
		cols := []interface{}{}

		name := v.name
		if len(v.name) > maxNameLength {
			name = v.name[:maxNameLength-12] + "..." + v.name[len(v.name)-9:]
		}

		indexes := "SKIPPED"
		if !skipKeyFrames {
			indexes = strings.Join(v.indexes, " ")
		}

		cols = append(cols, name)
		cols = append(cols, intToString(v.size, " ", "B"))
		cols = append(cols, intToString(v.bitRate, " ", "bit"))
		cols = append(cols, float64(int(v.length*10))/10)
		cols = append(cols, float64(int(v.frameRate*10))/10)
		cols = append(cols, v.width)
		cols = append(cols, v.height)
		cols = append(cols, v.codec)
		cols = append(cols, indexes)

		t.AddLine(cols...)
	}

	t.Print()
}

func intToString(n int64, s, s2 string) string {
	if n > 1000*1000*1000*1000 {
		return fmt.Sprintf("%.1f%sT%s", float64(n)/1000/1000/1000/1000, s, s2)
	} else if n > 1000*1000*1000 {
		return fmt.Sprintf("%.1f%sG%s", float64(n)/1000/1000/1000, s, s2)
	} else if n > 1000*1000 {
		return fmt.Sprintf("%.1f%sM%s", float64(n)/1000/1000, s, s2)
	} else if n > 1000 {
		return fmt.Sprintf("%.1f%sK%s", float64(n)/1000, s, s2)
	}

	return fmt.Sprintf("%d%s%s", n, s, s2)
}

func getBitRate(fi os.FileInfo) (int64, error) {
	bitrateRaw, err := exec(fmt.Sprintf("ffprobe -v quiet -select_streams v:0 -show_entries stream=bit_rate -of default=noprint_wrappers=1 %q", fi.Name()))
	if err != nil {
		return 0, fmt.Errorf("failed to probe file. file: %q, err: %w", fi.Name(), err)
	}

	if len(bitrateRaw) < 10 {
		return 0, fmt.Errorf("invalid probe result. file: %q, bitrate found: %s", fi.Name(), bitrateRaw)
	}

	bitrateRaw = strings.TrimSpace(bitrateRaw[9:])
	if bitrateRaw == "N/A" {
		return 0, nil
	}

	bitRate, err := strconv.ParseInt(bitrateRaw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse bit rate. file: %q, err: %w", fi.Name(), err)
	}

	return bitRate, nil
}

func getCodec(fi os.FileInfo) (string, error) {
	codec, err := exec(fmt.Sprintf("ffprobe -v quiet -select_streams v:0 -show_entries stream=codec_name -of default=noprint_wrappers=1:nokey=1 %q", fi.Name()))
	if err != nil {
		return "", fmt.Errorf("failed to probe file for codec. file: %q, err: %w", fi.Name(), err)
	}

	parts := strings.Split(strings.TrimSpace(codec), " ")
	if len(parts) > 1 {
		return "", fmt.Errorf("suspicious codec found. file: %q, codec: %s", fi.Name(), codec)
	}

	return parts[0], nil
}

func getLength(fi os.FileInfo) (float64, error) {
	lengthRaw, err := exec(fmt.Sprintf("ffprobe -v quiet -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 %q", fi.Name()))
	if err != nil {
		return 0.0, fmt.Errorf("failed to probe file for length. file: %q, err: %w", fi.Name(), err)
	}

	l, err := strconv.ParseFloat(strings.TrimSpace(lengthRaw), 64)
	if err != nil {
		return 0.0, fmt.Errorf("failed to parse length. file: %q, err: %w", fi.Name(), err)
	}

	return l, nil
}

func getFrameRate(fi os.FileInfo) (float64, error) {
	frameRateRaw, err := exec(fmt.Sprintf("ffprobe -v quiet -select_streams v -of default=noprint_wrappers=1:nokey=1 -show_entries stream=r_frame_rate %q", fi.Name()))
	if err != nil {
		return 0.0, fmt.Errorf("failed to probe file for frame rate. file: %q, err: %w", fi.Name(), err)
	}

	parts := strings.Split(strings.TrimSpace(frameRateRaw), "/")
	p0, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0.0, fmt.Errorf("failed to parse frame rate. file: %q, frame rate: %s, err: %w", fi.Name(), frameRateRaw, err)
	}
	p1, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0.0, fmt.Errorf("failed to parse frame rate. file: %q, frame rate: %s, err: %w", fi.Name(), frameRateRaw, err)
	}

	return p0 / p1, nil
}

func info(fi os.FileInfo, skipKeyFrames bool) videoType {
	bitRate, err := getBitRate(fi)
	if err != nil {
		l.Printf("failed to retrieve video bitrate. err: %q", err)
	}

	length, err := getLength(fi)
	if err != nil {
		l.Printf("failed to retrieve video length. err: %q", err)
	}

	frameRate, err := getFrameRate(fi)
	if err != nil {
		l.Printf("failed to retrieve video frame rate. err: %q", err)
	}

	dimensions, err := getDimensions(fi)
	if err != nil {
		l.Printf("failed to retrieve video dimensions. err: %q", err)
	}

	width, height, err := parseDimensions(dimensions)
	if err != nil {
		l.Printf("failed to parse video dimensions. err: %q", err)
	}

	codec, err := getCodec(fi)
	if err != nil {
		l.Printf("failed to retrieve video codec. err: %q", err)
	}

	var indexes []string
	if !skipKeyFrames {
		indexes, err = findKeyFrames(fi)
		if err != nil {
			l.Printf("failed to find key frames. err: %q", err)
		}
	}

	return videoType{
		name:      fi.Name(),
		size:      fi.Size(),
		bitRate:   bitRate,
		length:    length,
		frameRate: frameRate,
		width:     int64(width),
		height:    int64(height),
		codec:     codec,
		indexes:   indexes,
	}
}

func infoAll(fileList []os.FileInfo, skipKeyFrames bool, maxNameLength int) error {
	v := videoTypes{}
	for _, fi := range fileList {
		if fi.IsDir() {
			continue
		}

		v = append(v, info(fi, skipKeyFrames))
	}

	v.Print(skipKeyFrames, maxNameLength)

	return nil
}

func (a App) infoAll(c *cli.Context, args []string, fileList []os.FileInfo, dryRun bool) error {
	skipKeyFrames := c.Bool(skipKeyframesFlag)
	maxNameLength := c.Int(maxNameLengthFlag)

	return infoAll(fileList, skipKeyFrames, maxNameLength)
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
	cropArgsUsage = "[left|center|right|px from left] [top|center|bottom|px from top] [files...]"

	infoCommand   = "info"
	infoAliases   = "i"
	infoUsage     = "display info about the video(s). (The backwards flag is ignored.)"
	infoArgsUsage = "[files...]"

	datePrefixCommand   = "prefix-date"
	datePrefixAliases   = "pd"
	datePrefixUsage     = `add a date prefix to the file name`
	datePrefixArgsUsage = "[files...]"
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
	crfUsage = "crf to use for encoding (https://slhck.info/video/2017/02/24/crf-guide.html)"

	forceFlag  = "force-overwrite"
	forceAlias = "f"
	forceUsage = "force overwriting existing files"

	dimensionPresetFlag  = "dimension-preset"
	dimensionPresetAlias = "dp"
	dimensionPresetUsage = "preset to use for video dimensions"

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

	widthFlag  = "width"
	widthUsage = "width to use for cropping video"

	heightFlag  = "height"
	heightUsage = "height to use for cropping video"

	xFlag  = "x"
	xUsage = "x position to use for cropping video (number, left, center, right)"

	yFlag  = "y"
	yUsage = "y position to use for cropping video (number, top, center, bottom)"

	hwaccelFlag  = "hwaccel"
	hwaccelAlias = "hw"
	hwaccelUsage = "hardware acceleration to use for encoding [qsv]"

	hwaccelDeviceFlag  = "hwaccel_device"
	hwaccelDeviceAlias = "hwd"
	hwaccelDeviceUsage = "hardware acceleration to use for encoding [/dev/dri/renderD128]"

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

	skipDuplicateFlag  = "skip-duplicate"
	skipDuplicateAlias = "sd"
	skipDuplicateUsage = "if true, the text will not be added if it already exists"

	verboseFlag  = "verbose"
	verboseAlias = "v"
	verboseUsage = "print commands before executing them"

	skipKeyframesFlag  = "skip-keyframes"
	skipKeyframesAlias = "sk"
	skipKeyframesUsage = "if true, keyframes will not be included in the result"

	maxNameLengthFlag    = "maximum-name-length"
	maxNameLengthAlias   = "mnl"
	maxNameLengthUsage   = "maximum length of a file name"
	maxNameLengthDefault = 50

	replaceFileFlag  = "replace-file"
	replaceFileAlias = "rf"
	replaceFileUsage = "if true, the original file is backed up and replaced"
)

func main() {
	a := App{}

	globalFlags := map[string]cli.Flag{
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
		forceFlag: &cli.BoolFlag{
			Name:    forceFlag,
			Aliases: []string{forceAlias},
			Value:   false,
			Usage:   forceUsage,
		},
		verboseFlag: &cli.BoolFlag{
			Name:    verboseFlag,
			Aliases: []string{verboseAlias},
			Value:   false,
			Usage:   verboseUsage,
		},
	}

	commandFlags := map[string]cli.Flag{
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
		hwaccelFlag: &cli.StringFlag{
			Name:    hwaccelFlag,
			Aliases: []string{hwaccelAlias},
			Usage:   hwaccelUsage,
		},
		hwaccelDeviceFlag: &cli.StringFlag{
			Name:    hwaccelDeviceFlag,
			Aliases: []string{hwaccelDeviceAlias},
			Usage:   hwaccelDeviceUsage,
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
		skipDuplicateFlag: &cli.BoolFlag{
			Name:    skipDuplicateFlag,
			Aliases: []string{skipDuplicateAlias},
			Value:   false,
			Usage:   skipDuplicateUsage,
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
		skipKeyframesFlag: &cli.BoolFlag{
			Name:    skipKeyframesFlag,
			Aliases: []string{skipKeyframesAlias},
			Value:   false,
			Usage:   skipKeyframesUsage,
		},
		maxNameLengthFlag: &cli.IntFlag{
			Name:    maxNameLengthFlag,
			Aliases: []string{maxNameLengthAlias},
			Value:   maxNameLengthDefault,
			Usage:   maxNameLengthUsage,
		},
		dimensionPresetFlag: &cli.StringFlag{
			Name:    dimensionPresetFlag,
			Aliases: []string{dimensionPresetAlias},
			Usage:   dimensionPresetUsage,
		},
		widthFlag: &cli.IntFlag{
			Name:  widthFlag,
			Usage: widthUsage,
		},
		heightFlag: &cli.IntFlag{
			Name:  heightFlag,
			Usage: heightUsage,
		},
		xFlag: &cli.StringFlag{
			Name:  xFlag,
			Usage: xUsage,
		},
		yFlag: &cli.StringFlag{
			Name:  yFlag,
			Usage: yUsage,
		},
		replaceFileFlag: &cli.BoolFlag{
			Name:    replaceFileFlag,
			Aliases: []string{replaceFileAlias},
			Value:   false,
			Usage:   replaceFileUsage,
		},
	}

	app := &cli.App{
		Name: "ffr",
		Flags: []cli.Flag{
			globalFlags[backwardsFlag],
			globalFlags[dryRunFlag],
			globalFlags[forceFlag],
			globalFlags[verboseFlag],
		},
		Commands: []*cli.Command{
			{
				Name:      addNumberCommand,
				Aliases:   strings.Split(addNumberAliases, ", "),
				Usage:     addNumberUsage,
				ArgsUsage: addNumberArgsUsage,
				Flags: []cli.Flag{
					commandFlags[maxCountFlag],
					commandFlags[regexpFlag],
					commandFlags[regexpGroupFlag],
					commandFlags[skipFindsFlag],
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
					commandFlags[fromBackFlag],
					commandFlags[partsFlag],
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
					commandFlags[maxCountFlag],
					commandFlags[regexpFlag],
					commandFlags[regexpGroupFlag],
					commandFlags[skipPartsFlag],
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
					commandFlags[regexpFlag],
					commandFlags[skipDashPrefixFlag],
					commandFlags[skipDuplicateFlag],
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
					commandFlags[regexpFlag],
					commandFlags[skipDashPrefixFlag],
					commandFlags[skipDuplicateFlag],
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
				Flags:     []cli.Flag{},
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
					commandFlags[deleteTextFlag],
					commandFlags[regexpFlag],
					commandFlags[skipPartsFlag],
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
					commandFlags[skipPartsFlag],
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
					commandFlags[codecFlag],
					commandFlags[crfFlag],
					commandFlags[presetFlag],
					commandFlags[hwaccelFlag],
					commandFlags[hwaccelDeviceFlag],
					commandFlags[replaceFileFlag],
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
					commandFlags[skipFindsFlag],
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
					commandFlags[skipPartsFlag],
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
					commandFlags[widthFlag],
					commandFlags[heightFlag],
					commandFlags[xFlag],
					commandFlags[yFlag],
					commandFlags[dimensionPresetFlag],
				},
				Action: func(c *cli.Context) error {
					return process(c, 0, a.crop)
				},
			},
			{
				Name:      infoCommand,
				Aliases:   strings.Split(infoAliases, ", "),
				Usage:     infoUsage,
				ArgsUsage: infoArgsUsage,
				Flags: []cli.Flag{
					commandFlags[skipKeyframesFlag],
					commandFlags[maxNameLengthFlag],
				},
				Action: func(c *cli.Context) error {
					_ = c.Set(backwardsFlag, "0")

					return processAll(c, 0, a.infoAll)
				},
			},
			{
				Name:      datePrefixCommand,
				Aliases:   strings.Split(datePrefixAliases, ", "),
				Usage:     datePrefixUsage,
				ArgsUsage: datePrefixArgsUsage,
				Flags:     []cli.Flag{},
				Action: func(c *cli.Context) error {
					return process(c, 0, a.datePrefix)
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

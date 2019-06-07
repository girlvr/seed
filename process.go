package seed

import (
	"context"
	"crypto/md5"
	"fmt"
	cmd "github.com/godcong/go-ffmpeg-cmd"
	"os"
	"path"
	"path/filepath"
	"strings"

	shell "github.com/godcong/go-ipfs-restapi"
	"github.com/yinhevr/seed/model"
	"golang.org/x/xerrors"
	"gopkg.in/urfave/cli.v2"
)

// ProcessCallbackFunc ...
type ProcessCallbackFunc func(process *Process) error

// Process ...
type Process struct {
	Workspace string `json:"workspace"`
	Pin       bool   `json:"pin"`
	Slice     bool   `json:"slice"`
	Move      bool   `json:"move"`
	MovePath  string `json:"move_path"`
	JSON      bool   `json:"json"`
	JSONPath  string `json:"json_path"`
	Thread    int    `json:"thread"`
	Before    ProcessCallbackFunc
	After     ProcessCallbackFunc
	ignores   map[string][]byte
}

func initIgnore() map[string][]byte {
	return make(map[string][]byte, 3)
}

func tmp(path string, name string) string {
	mp, e := filepath.Abs(path)
	if e != nil {
		mp, e = filepath.Abs(filepath.Dir(os.Args[0]))
		if e != nil {
			//ignore error
			mp, _ = os.UserHomeDir()
		}
	}
	return filepath.Join(mp, name)
}

// NewProcess ...
func NewProcess(ws string) *Process {
	return &Process{
		Workspace: ws,
		Pin:       false,
		Slice:     true,
		Move:      true,
		MovePath:  tmp(ws, "success"),
		JSON:      false,
		JSONPath:  "seed.json",
		Before:    nil,
		After:     nil,
		ignores:   initIgnore(),
	}
}

func prefix(s string) (ret string) {
	ret = "/ipfs/" + s
	return
}

func isVideo(filename string) bool {
	vlist := []string{
		"mkv", ".mp4", ".mpg", ".mpeg", ".avi", ".rm", ".rmvb", ".mov", ".wmv", ".asf", ".dat", ".asx", ".wvx", ".mpe", ".mpa",
	}
	for _, v := range vlist {
		if path.Ext(filename) == v {
			return true
		}
	}
	return false
}

// CmdProcess ...
func CmdProcess(app *cli.App) *cli.Command {
	flags := append(app.Flags,
		&cli.BoolFlag{
			Name:        "quick",
			Aliases:     []string{"q"},
			Usage:       "process data to ipfs skip read json",
			Value:       false,
			Destination: nil,
		})

	return &cli.Command{
		Name:    "process",
		Aliases: []string{"P"},
		Usage:   "",
		Action: func(context *cli.Context) error {
			log.Info("process call")

			if context.Bool("q") {
				//QuickProcess()
			}
			//ProcessVideo()

			return nil
		},
		Subcommands: nil,
		Flags:       flags,
	}
}

// Run ...
func (p *Process) Run() {
	p.ignore()
	if p.Before != nil {
		if err := p.Before(p); err != nil {
			log.Error(err)
			return
		}
	}
	files := p.getFiles(p.Workspace)
	log.Info(files)
	for _, file := range files {
		log.Info(file)
		Unfinished := DefaultUnfinished(file)
		object, err := rest.AddFile(file)
		if err != nil {
			log.Error(err)
			continue
		}
		Unfinished.Hash = object.Hash
		Unfinished.Object = model.ObjectIntoLink(Unfinished.Object, object)
		//fix name and get format
		format, err := parseUnfinishedFromStreamFormat(file, Unfinished)
		if err != nil {
			log.Error(err)
			continue
		}
		log.Infof("%+v", format)
		err = model.AddOrUpdateUnfinished(Unfinished)
		if err != nil {
			log.Error(err)
			continue
		}

		if Unfinished.IsVideo && p.Slice {
			log.With("split", file).Info("process")
			sa, err := cmd.FFMpegSplitToM3U8(nil, file, cmd.StreamFormatOption(format))
			if err != nil {
				log.Error(err)
				continue
			}
			dirs, err := rest.AddDir(sa.Output)
			if err != nil {
				log.Error(err)
				continue
			}

			if len(dirs) > 0 {
				Unfinished.Hash = dirs[0].Hash
				Unfinished.Type = "m3u8"
			}
			Unfinished.Object = model.ObjectIntoLink(Unfinished.Object, dirs[0])
			Unfinished.Object.ParseLinks(dirs)
			err = model.AddOrUpdateUnfinished(Unfinished)
			if err != nil {
				log.Error(err)
				continue
			}
		}

	}

	return
}

// PathMD5 ...
func PathMD5(s ...string) string {
	str := filepath.Join(s...)
	return fmt.Sprintf("%x", md5.Sum([]byte(str)))
}

// Ignore ...
func (p *Process) ignore() {
	p.ignores[PathMD5(p.MovePath)] = nil
}

// CheckIgnore ...
func (p *Process) CheckIgnore(name string) (b bool) {
	if p.ignores == nil {
		return false
	}
	_, b = p.ignores[PathMD5(name)]
	return
}

func (p *Process) getFiles(ws string) (files []string) {
	info, e := os.Stat(ws)
	if e != nil {
		return nil
	}
	if info.IsDir() {
		file, e := os.Open(ws)
		if e != nil {
			return nil
		}
		defer file.Close()
		names, e := file.Readdirnames(-1)
		if e != nil {
			return nil
		}
		var fullPath string
		for _, name := range names {
			fullPath = filepath.Join(ws, name)
			if p.CheckIgnore(fullPath) {
				continue
			}
			tmp := p.getFiles(fullPath)
			if tmp != nil {
				files = append(files, tmp...)
			}
		}
		return files
	}
	return append(files, ws)
}

func parseUnfinishedFromStreamFormat(file string, u *model.Unfinished) (format *cmd.StreamFormat, e error) {
	format, e = cmd.FFProbeStreamFormat(file)
	if e != nil {
		return nil, e
	}

	if format.IsVideo() {
		u.IsVideo = true
		u.Type = "video"
		u.Name = format.NameAnalyze().ToString()
		u.Sharpness = getVideoResolution(format)
	}
	return format, nil
}

// DefaultUnfinished ...
func DefaultUnfinished(name string) *model.Unfinished {
	_, file := filepath.Split(name)
	uncat := &model.Unfinished{
		Model:       model.Model{},
		Checksum:    "",
		Type:        "other",
		Name:        file,
		Hash:        "",
		IsVideo:     false,
		Sync:        false,
		Sliced:      false,
		Encrypt:     false,
		Key:         "",
		M3U8:        "media.m3u8",
		SegmentFile: "media-%05d.ts",
		Caption:     "",
		Object:      nil,
	}
	uncat.Checksum = model.Checksum(name)
	return uncat
}

// QuickProcess ...
func QuickProcess(pathname string, needPin bool) (e error) {
	info, e := os.Stat(pathname)
	if e != nil {
		return e
	}
	b := info.IsDir()
	if b {
		file, e := os.Open(pathname)
		if e != nil {
			return e
		}
		defer file.Close()
		names, e := file.Readdirnames(-1)
		if e != nil {
			return e
		}
		for _, value := range names {
			uncat := model.Unfinished{
				Name:    value,
				Type:    "other",
				Hash:    "",
				IsVideo: false,
				Object:  nil,
			}
			file := filepath.Join(pathname, value)
			fileinfo, e := os.Stat(file)
			if e != nil {
				log.Error(e)
				continue
			}
			if fileinfo.IsDir() {
				log.Error(value, " continue with dir")
				continue
			}

			log.Infof("add [%s]:%s", uncat.Checksum, file)
			object, e := rest.AddFile(file)
			if e != nil {
				log.Errorf("add file error:%+v", object)
				continue
			}
			log.Info("added", uncat.Checksum)
			uncat.Hash = object.Hash
			//uncat.Object = append(uncat.Object, model.ObjectIntoLink(nil, object))
			uncat.IsVideo = isVideo(value)
			if uncat.IsVideo {
				uncat.Type = "video"
			}
			e = model.AddOrUpdateUnfinished(&uncat)
			if e != nil {
				log.Errorf("insert Unfinished error:%+v", e)
				continue
			}
			log.Info("inserted table", uncat)
			if uncat.IsVideo {
				uncatvideo := model.Unfinished{
					Model:    model.Model{},
					Checksum: uncat.Checksum,
					Type:     "m3u8",
					Name:     value,
					Hash:     "",
					IsVideo:  uncat.IsVideo,
					Object:   nil,
				}
				file, e := SplitVideo(context.Background(), nil, file)
				if e != nil {
					log.Errorf("split file error:%+v", object)
					continue
				}
				log.Info(file)
				rets, e := rest.AddDir(file)
				if e != nil {
					log.Errorf("ipfs add file error:%+v", object)
					continue
				}
				last := len(rets) - 1
				var obj *model.VideoObject
				for idx, v := range rets {
					if idx == last {
						obj = model.ObjectIntoLink(obj, v)
						uncatvideo.Hash = obj.Link.Hash
						continue
					}
					obj = model.ObjectIntoLinks(obj, v)
				}
				log.Infof("%+v", *obj)
				//uncatvideo.Object = append(uncatvideo.Object, obj)
				log.Info("inserted video table", uncatvideo)
				if needPin {
					pin(nil, uncatvideo.Hash, func(hash string) {
						uncatvideo.Sync = true
						e = model.AddOrUpdateUnfinished(&uncatvideo)
						if e != nil {
							log.Errorf("insert Unfinished error:%+v", e)
						}
					})
					continue
				}
				e = model.AddOrUpdateUnfinished(&uncatvideo)
				if e != nil {
					log.Errorf("insert Unfinished error:%+v", e)
					continue
				}
			}
			log.Info("move file", file)
			if err := moveSuccess(file); err != nil {
				return err
			}

		}

	}
	return nil
}

func moveSuccess(file string) (e error) {
	dir, name := filepath.Split(file)
	newPath := filepath.Join(dir, "success")
	_ = os.MkdirAll(newPath, os.ModePerm)
	newPathFile := filepath.Join(newPath, name)
	return os.Rename(file, newPathFile)
}

// ProcessVideo ...
func ProcessVideo(source *VideoSource) (e error) {
	if source == nil {
		return xerrors.New("nil source")
	}
	if source.Bangumi == "" {
		return nil
	}
	source.Bangumi = strings.ToUpper(source.Bangumi)
	video := &model.Video{}
	_, err := model.FindVideo(source.Bangumi, video, false)
	if err != nil {
		return err
	}
	parseVideoBase(video, source)

	log.Infof("%+v", video)

	video.PosterHash = addPosterHash(source)
	log.Info(*source)

	fn := add
	if source.CheckFiles != nil {
		log.Debug("add from Unfinished")
		fn = addChecksum
	} else if source.Slice {
		log.Debug("add with slice")
		fn = addSlice
	}
	e = fn(video, source)
	if e != nil {
		return e
	}
	info := GetSourceInfo()
	log.Info(*info)

	//if info.ID != "" {
	//	video.AddSourceInfo(info)
	//}
	//
	//for _, value := range GetPeers() {
	//	video.AddPeers(&model.SourcePeerDetail{
	//		Addr: value.Addr,
	//		Peer: value.Peer,
	//	})
	//}
	return model.AddOrUpdateVideo(video)
}

func addPosterHash(source *VideoSource) string {
	if source.PosterPath != "" {
		object, e := rest.AddFile(source.PosterPath)
		if e != nil {
			log.Error("add poster error:", e)
			return ""
		}
		return object.Hash
	}
	return source.Poster
}

// GetSourceInfo ...
func GetSourceInfo() *model.SourceInfoDetail {
	out, e := rest.ID()
	if e != nil {
		return &model.SourceInfoDetail{}
	}
	return (*model.SourceInfoDetail)(out)
}

// GetPeers ...
func GetPeers() []shell.SwarmConnInfo {
	swarmPeers, e := rest.SwarmPeers(context.Background())
	if e != nil {
		return nil
	}
	size := len(swarmPeers.Peers)
	if size == 0 {
		return nil
	}
	if size > 50 {
		size = 50
	}

	return swarmPeers.Peers[:size]
}

// MustString  must string
func MustString(val, src string) string {
	if val != "" {
		return val
	}
	return src
}

// hls ...
func initHLS(source *VideoSource) {
	if source != nil {
		source.Key = MustString(source.Key, "")
		source.M3U8 = MustString(source.M3U8, "media.m3u8")
		source.SegmentFile = MustString(source.SegmentFile, "media-%05d.ts")
	}
}

func addSlice(video *model.Video, source *VideoSource) (e error) {
	initHLS(source)
	source.Files = nil
	for _, value := range source.Files {
		file, e := SplitVideo(context.Background(), nil, value)
		if e != nil {
			return e
		}
		source.Files = append(source.Files, file)
	}
	e = add(video, source)
	if e != nil {
		return e
	}

	return nil
}

func addChecksum(video *model.Video, source *VideoSource) (e error) {
	hash := Hash(source)
	group := parseGroup(hash, source)
	for _, value := range source.CheckFiles {
		Unfinished, e := model.FindUnfinished(value, false)
		if e != nil {
			return e
		}
		group.Object = []*model.VideoObject{Unfinished.Object}
	}

	//create if null
	//if video.VideoGroupList == nil {
	//	video.VideoGroupList = []*model.VideoGroup{group}
	//}
	//
	////replace if found
	//for i, v := range video.VideoGroupList {
	//	if v.Index == group.Index {
	//		video.VideoGroupList[i] = group
	//		return nil
	//	}
	//}
	////append if not found
	//video.VideoGroupList = append(video.VideoGroupList, group)
	return nil
}

func add(video *model.Video, source *VideoSource) (e error) {
	hash := Hash(source)
	group := parseGroup(hash, source)
	for _, value := range source.Files {
		info, e := os.Stat(value)
		if e != nil {
			log.Error(e)
			continue
		}
		dir := info.IsDir()

		if dir {
			rets, e := rest.AddDir(value)
			if e != nil {
				log.Error(e)
				continue
			}
			last := len(rets) - 1
			var obj *model.VideoObject
			for idx, v := range rets {
				if idx == last {
					obj = model.ObjectIntoLink(obj, v)
					//group.Object = append(group.Object)
					continue
				}
				obj = model.ObjectIntoLinks(obj, v)
			}
			group.Object = append(group.Object, obj)

			continue
		}
		ret, e := rest.AddFile(value)
		if e != nil {
			log.Error(e)
			continue
		}
		//hash = ret.Hash
		group.Object = append(group.Object, model.ObjectIntoLink(nil, ret))
	}

	//create if null
	//if video.VideoGroupList == nil {
	//	video.VideoGroupList = []*model.VideoGroup{group}
	//}
	//
	////replace if found
	//for i, v := range video.VideoGroupList {
	//	if v.Index == group.Index {
	//		video.VideoGroupList[i] = group
	//		return nil
	//	}
	//}
	////append if not found
	//video.VideoGroupList = append(video.VideoGroupList, group)
	return nil
}

func parseGroup(index string, source *VideoSource) *model.VideoGroup {
	return &model.VideoGroup{
		Index: index,
		//Sharpness:    source.Sharpness,
		//Output:       source.Output,
		//Season:       source.Season,
		//TotalEpisode: source.TotalEpisode,
		//Episode:      source.Episode,
		//Language:     source.Language,
		//Caption:      source.Caption,
		Sliced: source.Slice,
		//HLS:          *hls(source.HLS),
		Object: nil,
	}

}

// GroupIndex ...
func GroupIndex(source *VideoSource, hash string) (s string) {
	//switch strings.ToLower(source.Group) {
	//case "bangumi":
	//	s = source.Bangumi
	//case "sharpness":
	//	s = source.Sharpness
	//case "hash":
	//	return hash
	//default:
	//	s = uuid.Must(uuid.NewRandom()).String()
	//}
	return
}

// Load ...
func Load(path string) []*VideoSource {
	var vs []*VideoSource
	e := ReadJSON(path, &vs)
	if e != nil {
		return nil
	}
	return vs
}

package task

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/glvd/seed"
	"github.com/glvd/seed/model"
	files "github.com/ipfs/go-ipfs-files"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/xormsharp/xorm"
)

// InfoType ...
type InfoType string

// InfoTypeNone ...
const InfoTypeNone InfoType = "none"

// InfoTypeJSON ...
const InfoTypeJSON InfoType = "json"

// InfoTypeBSON ...
const InfoTypeBSON InfoType = "bson"

// DefaultLimit ...
const DefaultLimit = 5000

// Information ...
type Information struct {
	InfoType     InfoType
	Path         string
	ResourcePath string
	ProcList     []string
	Start        int
	Limit        int
	Filter       []string
}

// NewInformation ...
func NewInformation() *Information {
	inf := &Information{
		InfoType: InfoTypeBSON,
		ProcList: nil,
		Start:    0,
		Limit:    DefaultLimit,
	}
	return inf
}

// CallTask ...
func (info *Information) CallTask(seeder seed.Seeder, t *seed.Task) error {
	select {
	case <-seeder.Context().Done():
		return nil
	default:
		e := SplitCall(seeder, info)
		if e != nil {
			return e
		}
	}

	return nil
}

func fixBson(s []byte) []byte {
	reg := regexp.MustCompile(`("_id")[ ]*[:][ ]*(ObjectId\(")[\w]{24}("\))[ ]*(,)[ ]*`)
	return reg.ReplaceAll(s, []byte(" "))
}

func video(source *VideoSource) (v *model.Video) {

	//always not null
	alias := *new([]string)
	aliasS := ""
	if source.Alias != nil && len(source.Alias) > 0 {
		alias = source.Alias
		aliasS = alias[0]
	}
	//always not null
	role := *new([]string)
	roleS := ""
	if source.Role != nil && len(source.Role) > 0 {
		role = source.Role
		roleS = role[0]
	}

	intro := source.Intro
	if intro == "" {
		intro = aliasS + " " + roleS
	}

	return &model.Video{
		FindNo:       strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(source.Bangumi, "-", ""), "_", "")),
		Bangumi:      strings.ToUpper(source.Bangumi),
		Type:         source.Type,
		Systematics:  source.Systematics,
		Sharpness:    source.Sharpness,
		Producer:     source.Producer,
		Language:     source.Language,
		Caption:      source.Caption,
		Intro:        intro,
		Alias:        alias,
		Role:         role,
		Director:     source.Director,
		Series:       source.Series,
		Tags:         source.Tags,
		Date:         source.Date,
		SourceHash:   source.SourceHash,
		Season:       seed.MustString(source.Season, "1"),
		Episode:      seed.MustString(source.Episode, "1"),
		TotalEpisode: seed.MustString(source.TotalEpisode, "1"),
		Format:       seed.MustString(source.Format, "2D"),
		Publisher:    source.Publisher,
		Length:       source.Length,
		MagnetLinks:  source.MagnetLinks,
		Uncensored:   source.Uncensored,
	}
}

// defaultUnfinished ...
func defaultUnfinished(name string) *model.Unfinished {
	_, file := filepath.Split(name)

	u := &model.Unfinished{
		Model:       model.Model{},
		Checksum:    "",
		Type:        "other",
		Relate:      "",
		Name:        file,
		Hash:        "",
		Sharpness:   "",
		Caption:     "",
		Encrypt:     false,
		Key:         "",
		M3U8:        "media.m3u8",
		SegmentFile: "media-%05d.ts",
		Sync:        false,
		Object:      new(model.VideoObject),
	}
	u.Checksum = model.Checksum(name)
	log.With("file", name, "checksum", u.Checksum).Info("unfinished")
	return u
}

func checkFileNotExist(path string) bool {
	_, e := os.Stat(path)
	if e != nil {
		if os.IsNotExist(e) {
			return true
		}
		return false
	}
	return false
}

//
//// Run ...
//func (info *Information) Run(ctx context.Context) {
//	log.Info("information running")
//
//InfoEnd:
//	for {
//		select {
//		case <-ctx.Done():
//			break InfoEnd
//		case cb := <-info.cb:
//			if cb == nil {
//				break InfoEnd
//			}
//			info.SetState(StateRunning)
//			e := cb.Call(info)
//			if e != nil {
//				log.Error(e)
//			}
//		case <-time.After(30 * time.Second):
//			log.Info("info time out")
//			info.SetState(StateWaiting)
//		}
//	}
//	close(info.cb)
//	info.Finished()
//
//}

func addThumbHash(a *seed.API, api *httpapi.HttpApi, source *VideoSource) (unf *model.Unfinished, e error) {
	if a.Failed() {
		return nil, errors.New("ipfs failed")
	}
	file, e := os.Open(source.Thumb)
	if e != nil {
		return nil, e
	}
	resolved, e := api.Unixfs().Add(context.Background(), files.NewReaderFile(file), func(settings *options.UnixfsAddSettings) error {
		settings.Pin = true
		return nil
	})
	if e != nil {
		a.SetFailed(true)
		return nil, e
	}
	unfinThumb := defaultUnfinished(source.Thumb)
	unfinThumb.Type = model.TypeThumb
	unfinThumb.Relate = source.Bangumi
	if source.Thumb != "" {
		unfinThumb.Hash = model.PinHash(resolved)
		e = a.PushTo(seed.DatabaseCallback(unfinThumb, func(database *seed.Database, eng *xorm.Engine, v interface{}) (e error) {
			return model.AddOrUpdateUnfinished(eng.Where(""), v.(*model.Unfinished))
		}))
		if e != nil {
			return nil, e
		}
		return unfinThumb, nil
	}

	return nil, errors.New("no thumb")
}

func addPosterHash(a *seed.API, api *httpapi.HttpApi, source *VideoSource) (unf *model.Unfinished, e error) {
	if a.Failed() {
		return nil, errors.New("ipfs failed")
	}

	file, err := os.Open(source.PosterPath)
	if err != nil {
		return nil, err
	}
	resolved, err := api.Unixfs().Add(context.Background(), files.NewReaderFile(file))
	if err != nil {
		a.SetFailed(true)
		return nil, err
	}

	unfinPoster := defaultUnfinished(source.PosterPath)
	unfinPoster.Type = model.TypePoster
	unfinPoster.Relate = source.Bangumi

	if source.PosterPath != "" {
		unfinPoster.Hash = model.PinHash(resolved)
		e = a.PushTo(seed.DatabaseCallback(unfinPoster, func(database *seed.Database, eng *xorm.Engine, v interface{}) (e error) {
			return model.AddOrUpdateUnfinished(eng.Where(""), v.(*model.Unfinished))
		}))
		if e != nil {
			return nil, e
		}
		return unfinPoster, nil
	}
	return nil, errors.New("no poster")
}

func addVideo(a *seed.API, api *httpapi.HttpApi, video *model.Video, path string) {
	//file, e := os.Open(source.PosterPath)
	//if e != nil {
	//	return nil, e
	//}
	//resolved, e := api.Unixfs().Add(context.Background(), files.NewReaderFile(file))
	//if e != nil {
	//	return nil, e
	//}
	//
	//unfinPoster := defaultUnfinished(sou)
	//unfinPoster.Type = model.TypePoster
	//unfinPoster.Relate = source.Bangumi
	//
	//if source.PosterPath != "" {
	//	unfinPoster.Hash = model.PinHash(resolved)
	//	e = a.PushTo(DatabaseCallback(unfinPoster, func(database *Database, eng *xorm.Engine, v interface{}) (e error) {
	//		return model.AddOrUpdateUnfinished(eng.NewSession(), v.(*model.Unfinished))
	//	}))
	//	if e != nil {
	//		return nil, e
	//	}
	//	return unfinPoster, nil
	//}
	//return nil, xerrors.New("no poster")
}

func filterProcList(sources []*VideoSource, filterList []string) <-chan *VideoSource {
	vs := make(chan *VideoSource)
	if filterList == nil || len(filterList) == 0 {
		go func(vs chan<- *VideoSource) {
			defer func() {
				vs <- nil
			}()
			for _, source := range sources {
				vs <- source
			}
		}(vs)
	} else {
		log.With("list", filterList).Info("filter")
		go func(vs chan<- *VideoSource) {
			defer func() {
				vs <- nil
			}()
			for _, source := range sources {
				for _, v := range filterList {
					if source.Bangumi == v {
						vs <- source
					}
				}
			}
		}(vs)
	}

	return vs
}

// VideoSource ...
type VideoSource struct {
	Bangumi      string         `json:"bangumi"`       //番号 no
	SourceHash   string         `json:"source_hash"`   //原片hash
	Type         string         `json:"type"`          //类型：film，FanDrama
	Format       string         `json:"format"`        //输出：3D，2D
	VR           string         `json:"vr"`            //VR格式：左右，上下，平面
	Thumb        string         `json:"thumb"`         //缩略图
	Intro        string         `json:"intro"`         //简介 title
	Alias        []string       `json:"alias"`         //别名，片名
	VideoEncode  string         `json:"video_encode"`  //视频编码
	AudioEncode  string         `json:"audio_encode"`  //音频编码
	Files        []string       `json:"files"`         //存放路径
	HashFiles    []string       `json:"hash_files"`    //已上传Hash
	CheckFiles   []string       `json:"check_files"`   //Unfinished checksum
	Slice        bool           `json:"sliceAdd"`      //是否HLS切片
	Encrypt      bool           `json:"encrypt"`       //加密
	Key          string         `json:"key"`           //秘钥
	M3U8         string         `json:"m3u8"`          //M3U8名
	SegmentFile  string         `json:"segment_file"`  //ts切片名
	PosterPath   string         `json:"poster_path"`   //海报路径
	Poster       string         `json:"poster"`        //海报HASH
	ExtendList   []*seed.Extend `json:"extend_list"`   //扩展信息
	Role         []string       `json:"role"`          //角色列表 stars
	Director     string         `json:"director"`      //导演
	Systematics  string         `json:"systematics"`   //分级
	Season       string         `json:"season"`        //季
	Episode      string         `json:"episode"`       //集数
	TotalEpisode string         `json:"total_episode"` //总集数
	Sharpness    string         `json:"sharpness"`     //清晰度
	Publish      string         `json:"publish"`       //发行日
	Date         string         `json:"date"`          //发行日
	Length       string         `json:"length"`        //片长
	Producer     string         `json:"producer"`      //制片商
	Series       string         `json:"series"`        //系列
	Tags         []string       `json:"tags"`          //标签
	Publisher    string         `json:"publisher"`     //发行商
	Language     string         `json:"language"`      //语言
	Caption      string         `json:"caption"`       //字幕
	MagnetLinks  []string       `json:"magnet_links"`  //磁链
	Uncensored   bool           `json:"uncensored"`    //有码,无码
}

//// InformationOption ...
//func informationOption(info *Information) Options {
//	return func(seed Seeder) {
//		seed.SetThread(StepperInformation, info)
//	}
//}

func bsonVideoSource(path string) (vs []*VideoSource, e error) {
	var bs []byte
	bs, e = ioutil.ReadFile(path)
	if e != nil {
		return nil, e
	}
	fixed := fixBson(bs)
	reader := bytes.NewBuffer(fixed)
	vs = *new([]*VideoSource)
	e = LoadFrom(&vs, reader)
	return
}

func jsonVideoSource(path string) (vs []*VideoSource, e error) {
	var bs []byte
	bs, e = ioutil.ReadFile(path)
	if e != nil {
		return nil, e
	}
	reader := bytes.NewBuffer(bs)
	vs = *new([]*VideoSource)
	e = LoadFrom(&vs, reader)
	return
}

var infoCallList = map[InfoType]func(string) ([]*VideoSource, error){
	InfoTypeBSON: bsonVideoSource,
	InfoTypeJSON: jsonVideoSource,
}

// InformationProcessFunction ...
type InformationProcessFunction func(string) ([]*VideoSource, error)

type informationProcess struct {
	infoType     InfoType
	resourcePath string
	fn           InformationProcessFunction
	path         string
	list         []string
	start        int
}

// SplitCall ...
func SplitCall(seeder seed.Seeder, information *Information) (e error) {
	var vs []*VideoSource
	fn, b := infoCallList[information.InfoType]
	if b {
		vs, e = fn(information.Path)
		if e != nil {
			return e
		}
	}
	if vs == nil {
		return errors.New("no video source")
	}

	size := len(vs)
	var vstmp []*VideoSource
	if size > information.Limit {
		for i := 0; i < size; i += information.Limit {
			dir, file := filepath.Split(information.Path)
			open := filepath.Join(dir, file+"."+strconv.Itoa(i))

			openFile, e := os.OpenFile(open, os.O_CREATE|os.O_SYNC|os.O_RDWR|os.O_TRUNC, os.ModePerm)
			if e != nil {
				log.Error(e)
				continue
			}

			if i+information.Limit >= size {
				vstmp = vs[i:size]
			} else {
				vstmp = vs[i : i+information.Limit]
			}
			encoder := json.NewEncoder(openFile)
			e = encoder.Encode(vstmp)
			openFile.Close()
			if e != nil {
				log.Error(e)
				continue
			}

			newinfo := information.Clone()
			newinfo.Path = open
			e = seeder.PushTo(newinfo.ProcessCall())
			if e != nil {
				log.Error(e)
				continue
			}
			log.With("path", open).Info("json")
		}
	}
	return
}

func splitCall(seeder seed.Seeder, c *informationProcess, vs []*VideoSource, limit int) (b bool) {
	size := len(vs)
	var vstmp []*VideoSource
	if size > limit {
		for i := 0; i < size; i += limit {
			dir, file := filepath.Split(c.path)
			open := filepath.Join(dir, file+"."+strconv.Itoa(i))

			openFile, e := os.OpenFile(open, os.O_CREATE|os.O_SYNC|os.O_RDWR, os.ModePerm)
			if e != nil {
				log.Error(e)
				continue
			}
			defer openFile.Close()
			if i+limit >= size {
				vstmp = vs[i:size]
			} else {
				vstmp = vs[i : i+limit]
			}
			encoder := json.NewEncoder(openFile)
			e = encoder.Encode(vstmp)
			//openFile.Close()
			if e != nil {
				log.Error(e)
				continue
			}
			info := &Information{
				InfoType: InfoTypeJSON,
				Path:     open,
				ProcList: c.list,
			}
			e = seeder.PushTo(info.ProcessCall())
			if e != nil {
				log.Error(e)
				continue
			}
		}
		return true
	}
	return false
}

// Call ...
func (i *informationProcess) Call(process *seed.Process) error {
	log.Info("information call")
	var e error
	var vs []*VideoSource
	vs, e = i.fn(i.path)
	if e != nil {
		return e
	}
	if vs == nil {
		return errors.New("no video source")
	}

	vsc := filterProcList(vs, i.list)
	log.With("path", i.path, "size", len(vs)).Info("info")
	for {
		source := <-vsc
		if source == nil {
			return nil
		}
		v := video(source)
		if source.Poster != "" {
			v.PosterHash = source.Poster
		} else {
			if source.PosterPath != "" {
				source.PosterPath = filepath.Join(i.resourcePath, source.PosterPath)
				if checkFileNotExist(source.PosterPath) {
					log.With("bangumi", source.Bangumi, "path", source.PosterPath).Info("poster not found")
				} else {

					e := process.PushTo(seed.APICallback(source, func(api *seed.API, api2 *httpapi.HttpApi, v interface{}) (e error) {
						_, e = addPosterHash(api, api2, v.(*VideoSource))
						if e != nil {
							return e
						}

						return nil
					}))
					if e != nil {
						log.Error(e)
						continue
					}

				}
			}
		}

		if source.Thumb != "" {
			source.Thumb = filepath.Join(i.resourcePath, source.Thumb)
			if checkFileNotExist(source.Thumb) {
				log.With("bangumi", source.Bangumi, "path", source.PosterPath).Info("thumb not found")
			} else {
				e := process.PushTo(seed.APICallback(source, func(api *seed.API, api2 *httpapi.HttpApi, v interface{}) (e error) {
					_, e = addThumbHash(api, api2, v.(*VideoSource))
					if e != nil {
						return e
					}
					return nil
				}))
				if e != nil {
					log.Error(e)
					continue
				}
			}
		}
		e := process.PushTo(seed.DatabaseCallback(v, func(database *seed.Database, eng *xorm.Engine, v interface{}) (e error) {
			return model.AddOrUpdateVideo(eng.Where(""), v.(*model.Video))
		}))
		if e != nil {
			log.With("bangumi", v.Bangumi).Error(e)
		}
	}
}

// Task ...
func (info *Information) Task() *seed.Task {
	return seed.NewTask(info)
}

// ProcessCall ...
func (info *Information) ProcessCall() (seed.Stepper, seed.ProcessCaller) {
	fn, b := infoCallList[info.InfoType]
	if !b {
		fn = jsonVideoSource
	}
	return seed.StepperProcess, &informationProcess{
		infoType:     info.InfoType,
		fn:           fn,
		resourcePath: info.ResourcePath,
		path:         info.Path,
		list:         info.ProcList,
		start:        info.Start,
	}
}

// Clone ...
func (info *Information) Clone() (newinfo *Information) {
	newinfo = new(Information)
	*newinfo = *info
	return
}

var _ seed.ProcessCaller = &informationProcess{}

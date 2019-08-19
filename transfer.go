package seed

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/glvd/seed/model"
	"github.com/glvd/seed/old"
	"github.com/go-xorm/xorm"
	shell "github.com/godcong/go-ipfs-restapi"
	"golang.org/x/xerrors"
)

// TransferStatus ...
type TransferStatus string

// TransferFlagNone ...
const (
	// TransferStatusNone ...
	TransferStatusNone TransferStatus = "none"
	// TransferFlagVerify ...
	TransferFlagVerify TransferStatus = "verify"
	// TransferStatusToJSON ...
	TransferStatusToJSON TransferStatus = "json"
	// TransferStatusFromOther ...
	TransferStatusFromOther TransferStatus = "other"
	// TransferStatusFromOld ...
	TransferStatusFromOld TransferStatus = "old"
	// TransferStatusUpdate ...
	TransferStatusUpdate TransferStatus = "update"
	// TransferStatusDelete ...
	TransferStatusDelete TransferStatus = "delete"
)

// transfer ...
type transfer struct {
	shell      *shell.Shell
	unfinished map[string]*model.Unfinished
	videos     map[string]*model.Video
	workspace  string
	status     TransferStatus
	path       string
	reader     io.Reader
}

// BeforeRun ...
func (transfer *transfer) BeforeRun(seed *Seed) {
	transfer.shell = seed.Shell
	transfer.workspace = seed.Workspace
	transfer.unfinished = seed.Unfinished
	transfer.videos = seed.Videos

}

// AfterRun ...
func (transfer *transfer) AfterRun(seed *Seed) {
	seed.Videos = transfer.videos
	seed.Unfinished = transfer.unfinished
}

// TransferOption ...
func TransferOption(t *transfer) Options {
	return func(seed *Seed) {
		seed.thread[StepperTransfer] = t
	}
}

// Transfer ...
func Transfer(path string, from InfoFlag, status TransferStatus) Options {
	t := &transfer{
		path:   path,
		flag:   from,
		status: status,
	}
	return TransferOption(t)
}

func insertOldToUnfinished(ban string, obj *old.Object) error {
	hash := ""
	if obj.Link != nil {
		hash = obj.Link.Hash
	}
	unf := &model.Unfinished{
		Checksum:    hash,
		Type:        model.TypeSlice,
		Relate:      ban,
		Name:        ban,
		Hash:        hash,
		Sharpness:   "",
		Caption:     "",
		Encrypt:     false,
		Key:         "",
		M3U8:        "",
		SegmentFile: "",
		Sync:        false,
		Object:      ObjectFromOld(obj),
	}
	return model.AddOrUpdateUnfinished(unf)

}

// ObjectFromOld ...
func ObjectFromOld(obj *old.Object) *model.VideoObject {
	return &model.VideoObject{
		Links: obj.Links,
		Link:  obj.Link,
	}
}

func oldToVideo(source *old.Video) *model.Video {
	//always not null
	alias := []string{}
	aliasS := ""
	if source.Alias != nil && len(source.Alias) > 0 {
		alias = source.Alias
		aliasS = alias[0]
	}
	//always not null
	role := []string{}
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
		Intro:        intro,
		Alias:        alias,
		Role:         role,
		Season:       "1",
		Episode:      "1",
		TotalEpisode: "1",
		Format:       "2D",
	}

}

func transferFromOld(engine *xorm.Engine) (e error) {
	videos := old.AllVideos(engine)
	log.With("size", len(videos)).Info("videos")
	for _, v := range videos {
		obj := old.GetObject(v)
		e := insertOldToUnfinished(v.Bangumi, obj)
		if e != nil {
			log.With("bangumi", v.Bangumi).Error(e)
			continue
		}
		vd, e := model.FindVideo(nil, v.Bangumi)
		if e != nil {
			log.With("bangumi", v.Bangumi).Error(e)
			continue
		}

		log.With("bangumi", v.Bangumi, "v", vd).Info("v update")
		if vd.ID == "" {
			vd = oldToVideo(v)
		}

		if strings.TrimSpace(vd.M3U8Hash) == "" && obj.Link != nil {
			log.With("hash:", obj.Link.Hash, "bangumi", v.Bangumi).Info("info")
			vd.M3U8Hash = obj.Link.Hash
			e = model.AddOrUpdateVideo(vd)
			if e != nil {
				log.With("bangumi", v.Bangumi).Error(e)
				continue
			}
		} else {

		}

	}
	return e
}

func copyUnfinished(from *xorm.Engine) (e error) {
	i, e := from.Count(&model.Unfinished{})
	if e != nil {
		log.Error(e)
		return
	}

	unfChan := make(chan *model.Unfinished, 5)
	go func(unfin chan<- *model.Unfinished) {
		for x := int64(0); x < i; x += 5 {
			unfs := new([]*model.Unfinished)
			e := from.Limit(5, int(x)).Find(unfs)
			if e != nil {
				continue
			}
			for _, u := range *unfs {
				unfin <- u
			}
		}
		unfin <- nil
	}(unfChan)

	for {
		select {
		case unfin := <-unfChan:
			if unfin == nil {
				goto END
			}
			unfin.ID = ""
			unfin.Version = 0
			e := model.AddOrUpdateUnfinished(unfin)
			log.With("checksum", unfin.Checksum, "type", unfin.Type, "relate", unfin.Relate, "error", e).Info("copy")
			if e != nil {
				return e
			}
		}
	}

END:
	log.Infof("unfinished(%d) done", i)
	return nil
}

func transferFromOther(engine *xorm.Engine) (e error) {
	if err := copyUnfinished(engine); err != nil {
		return err
	}

	fromList := new([]*model.Video)
	e = engine.Find(fromList)
	if e != nil {
		return
	}
	for _, from := range *fromList {
		video, e := model.FindVideo(nil, from.Bangumi)
		if e != nil {
			log.Error(e)
			continue
		}
		if video.ID == "" {
			continue
		}
		video.M3U8Hash = MustString(from.M3U8Hash, video.M3U8Hash)
		video.Sharpness = MustString(from.Sharpness, video.Sharpness)
		video.SourceHash = MustString(from.SourceHash, video.SourceHash)
		if video.M3U8Hash == "" {
			video.Season = from.Season
			video.Episode = from.Episode
			video.TotalEpisode = from.TotalEpisode
		}

		e = model.AddOrUpdateVideo(video)
		if e != nil {
			log.With("bangumi", from.Bangumi).Error(e)
			continue
		}
	}
	return e
}

func transferUpdate(engine *xorm.Engine) (e error) {
	fromList := new([]*model.Unfinished)
	e = engine.Find(fromList)
	if e != nil {
		return
	}
	for _, from := range *fromList {
		video, e := model.FindVideo(model.DB().Where("episode = ?", NumberIndex(from.Relate)), onlyName(from.Relate))
		if e != nil {
			log.Error(e)
			continue
		}

		if from.Type == model.TypeSlice {
			video.Sharpness = MustString(from.Sharpness, video.Sharpness)
			video.M3U8Hash = MustString(from.Hash, video.M3U8Hash)
		} else if from.Type == model.TypeVideo {
			video.Sharpness = MustString(from.Sharpness, video.Sharpness)
			video.SourceHash = MustString(from.Hash, video.SourceHash)
		} else {

		}
		e = model.AddOrUpdateVideo(video)
		if e != nil {
			log.With("bangumi", video.Bangumi, "index", video.Episode).Error(e)
			continue
		}
	}
	return e
}

func transferToJSON(to string) (e error) {
	videos, e := model.AllVideos(model.DB().Where("m3u8_hash <> ?", ""), 0)
	if e != nil {
		return e
	}
	bytes, e := json.Marshal(videos)
	if e != nil {
		return e
	}
	file, e := os.OpenFile(to, os.O_CREATE|os.O_SYNC|os.O_RDWR, os.ModePerm)
	if e != nil {
		return e
	}
	defer file.Close()
	n, e := file.Write(bytes)
	log.With("video", len(*videos)).Infof("write(%d)", n)
	return e
}

// Run ...
func (transfer *transfer) Run(ctx context.Context) {
	if transfer.flag == InfoFlagSQLite {
		fromDB, e := model.InitSQLite3(transfer.path)
		if e != nil {
			log.Error(e)
			return
		}
		e = fromDB.Sync2(model.Video{})
		if e != nil {
			log.Error(e)
			return
		}
		switch transfer.status {
		case TransferStatusFromOld:
			if err := transferFromOld(fromDB); err != nil {
				log.Error(err)
				return
			}
		//update flag video flag other sqlite3
		case TransferStatusFromOther:

			if err := transferFromOther(fromDB); err != nil {
				return
			}
		//update flag unfinished flag other sqlite3
		case TransferStatusUpdate:
			if err := transferUpdate(fromDB); err != nil {
				return
			}
		}
	} else if transfer.flag == InfoFlagJSON {
		switch transfer.status {
		case TransferStatusToJSON:
			if err := transferToJSON(transfer.path); err != nil {
				return
			}
		}
	}

}

// LoadFrom ...
func LoadFrom(vs *[]*VideoSource, reader io.Reader) (e error) {
	dec := json.NewDecoder(bufio.NewReader(reader))
	return dec.Decode(vs)
}

// TransferTo ...
func TransferTo(eng *xorm.Engine, limit int) (e error) {
	i, e := model.DB().Count(&model.Video{})
	if e != nil || i <= 0 {
		return e
	}
	if limit == 0 {
		limit = 10
	}
	for x := 0; x <= int(i); x += limit {
		var videos []*model.Video
		if e = model.DB().Limit(limit, x).Find(&videos); e != nil {
			return xerrors.Errorf("transfer error with:%d,%+v", x, e)
		}
		for _, v := range videos {
			log.Info("get:", v.Bangumi)
		}
		insert, e := eng.Insert(videos)
		log.Info(insert, e)
	}

	return nil
}

package model

import (
	"errors"

	"github.com/xormsharp/xorm"
)

// Type ...
type Type string

// TypeOther ...
const TypeOther Type = "other"

// TypeVideo ...
const TypeVideo Type = "video"

// TypeSlice ...
const TypeSlice Type = "slice"

// TypePoster ...
const TypePoster Type = "poster"

// TypeThumb ...
const TypeThumb Type = "thumb"

// TypeCaption caption file
const TypeCaption Type = "caption"

// Unfinished 未分类
type Unfinished struct {
	Model       `xorm:"extends"`
	Checksum    string       `xorm:"default() checksum"`            //sum值
	Type        Type         `xorm:"default() type"`                //类型
	Relate      string       `xorm:"default()" json:"relate"`       //关联信息
	Name        string       `xorm:"default() name"`                //名称
	Hash        string       `xorm:"default() hash"`                //哈希地址
	Sharpness   string       `xorm:"default()" json:"sharpness"`    //清晰度
	Caption     string       `xorm:"default()" json:"caption"`      //字幕
	Encrypt     bool         `json:"encrypt"`                       //加密
	Key         string       `xorm:"default()" json:"key"`          //秘钥
	M3U8        string       `xorm:"m3u8 default()" json:"m3u8"`    //M3U8名
	SegmentFile string       `xorm:"default()" json:"segment_file"` //ts切片名
	Sync        bool         `xorm:"notnull default(0)"`            //是否已同步
	Object      *VideoObject `xorm:"json" json:"object,omitempty"`  //视频信息
}

// GetID ...
func (unfin *Unfinished) GetID() string {
	return unfin.ID
}

// SetID ...
func (unfin *Unfinished) SetID(s string) {
	unfin.ID = s
}

// GetVersion ...
func (unfin *Unfinished) GetVersion() int {
	return unfin.Version
}

// SetVersion ...
func (unfin *Unfinished) SetVersion(i int) {
	unfin.Version = i
}

func init() {
	RegisterTable(Unfinished{})
}

// AllUnfinished ...
func AllUnfinished(session *xorm.Session, limit int, start ...int) (unfins *[]*Unfinished, e error) {
	unfins = new([]*Unfinished)
	session = MustSession(session)
	if limit > 0 {
		session = session.Limit(limit, start...)
	}
	if err := session.Find(unfins); err != nil {
		return nil, err
	}
	return unfins, nil
}

// FindUnfinished ...
func FindUnfinished(session *xorm.Session, checksum string) (unfin *Unfinished, e error) {
	unfin = new(Unfinished)
	b, e := MustSession(session).Where("checksum = ?", checksum).Get(unfin)
	if e != nil || !b {
		return nil, errors.New("unfinished not found")
	}
	return unfin, nil
}

// AddOrUpdateUnfinished ...
func AddOrUpdateUnfinished(session *xorm.Session, unfin *Unfinished) (e error) {
	tmp := new(Unfinished)
	var found bool
	session = MustSession(session)
	if unfin.ID != "" {
		found, e = session.Clone().ID(unfin.ID).Get(tmp)
	} else {
		found, e = session.Clone().Where("checksum = ?", unfin.Checksum).
			Where("type = ?", unfin.Type).Get(tmp)
	}
	if e != nil {
		return e
	}
	if found {
		//only slice need update,video update for check , hash changed
		i := int64(0)
		if unfin.Hash != unfin.Hash || unfin.Type == TypeSlice || unfin.Type == TypeVideo {
			unfin.Version = tmp.Version
			unfin.ID = tmp.ID
			i, e = session.Clone().ID(unfin.ID).Update(unfin)
			log.Infof("updated(%d): %+v", i, tmp)
		}
		return e
	}
	_, e = session.Clone().InsertOne(unfin)
	return
}

// Clone ...
func (unfin *Unfinished) Clone() (n *Unfinished) {
	n = new(Unfinished)
	*n = *unfin
	n.ID = ""
	n.Object = new(VideoObject)
	return
}

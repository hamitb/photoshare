package api

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/coopernurse/gorp"
	_ "github.com/lib/pq"
	"log"
	"os"
	"strings"
)

var (
	ErrInvalidLogin = errors.New("Invalid email or password")
)

func InitDB(db *sql.DB, logSql bool) (*gorp.DbMap, error) {
	dbMap := &gorp.DbMap{Db: db, Dialect: gorp.PostgresDialect{}}

	if logSql {
		dbMap.TraceOn("[sql]", log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds))
	}

	dbMap.AddTableWithName(User{}, "users").SetKeys(true, "ID")
	dbMap.AddTableWithName(Photo{}, "photos").SetKeys(true, "ID")
	dbMap.AddTableWithName(Tag{}, "tags").SetKeys(true, "ID")

	return dbMap, nil
}

type PhotoManager interface {
	Insert(*Photo) error
	Update(*Photo) error
	Delete(*Photo) error
	Get(int64) (*Photo, error)
	GetDetail(int64, *User) (*PhotoDetail, error)
	GetTagCounts() ([]TagCount, error)
	All(*Page, string) (*PhotoList, error)
	ByOwnerID(*Page, int64) (*PhotoList, error)
	Search(*Page, string) (*PhotoList, error)
	UpdateTags(*Photo) error
}

type defaultPhotoManager struct {
	dbMap *gorp.DbMap
}

func NewPhotoManager(dbMap *gorp.DbMap) PhotoManager {
	return &defaultPhotoManager{dbMap}
}

func (mgr *defaultPhotoManager) Delete(photo *Photo) error {
	_, err := mgr.dbMap.Delete(photo)
	return err
}

func (mgr *defaultPhotoManager) Update(photo *Photo) error {
	_, err := mgr.dbMap.Update(photo)
	return err
}

func (mgr *defaultPhotoManager) Insert(photo *Photo) error {
	t, err := mgr.dbMap.Begin()
	if err != nil {
		return err
	}
	if err := mgr.dbMap.Insert(photo); err != nil {
		return err
	}
	if err := mgr.UpdateTags(photo); err != nil {
		return err
	}
	return t.Commit()
}

func (mgr *defaultPhotoManager) UpdateTags(photo *Photo) error {

	var (
		args    = []string{"$1"}
		params  = []interface{}{interface{}(photo.ID)}
		isEmpty = true
	)
	for i, name := range photo.Tags {
		name = strings.TrimSpace(name)
		if name != "" {
			args = append(args, fmt.Sprintf("$%d", i+2))
			params = append(params, interface{}(strings.ToLower(name)))
			isEmpty = false
		}
	}
	if isEmpty && photo.ID != 0 {
		_, err := mgr.dbMap.Exec("DELETE FROM photo_tags WHERE photo_id=$1", photo.ID)
		return err
	}
	_, err := mgr.dbMap.Exec(fmt.Sprintf("SELECT add_tags(%s)", strings.Join(args, ",")), params...)
	return err

}

func (mgr *defaultPhotoManager) Get(photoID int64) (*Photo, error) {

	photo := &Photo{}

	if photoID == 0 {
		return photo, sql.ErrNoRows
	}

	obj, err := mgr.dbMap.Get(photo, photoID)
	if err != nil {
		return photo, err
	}
	if obj == nil {
		return photo, sql.ErrNoRows
	}
	return obj.(*Photo), nil
}

func (mgr *defaultPhotoManager) GetDetail(photoID int64, user *User) (*PhotoDetail, error) {

	photo := &PhotoDetail{}

	if photoID == 0 {
		return photo, sql.ErrNoRows
	}

	q := "SELECT p.*, u.name AS owner_name " +
		"FROM photos p JOIN users u ON u.id = p.owner_id " +
		"WHERE p.id=$1"

	if err := mgr.dbMap.SelectOne(photo, q, photoID); err != nil {
		return photo, err
	}

	var tags []Tag

	if _, err := mgr.dbMap.Select(&tags,
		"SELECT t.* FROM tags t JOIN photo_tags pt ON pt.tag_id=t.id "+
			"WHERE pt.photo_id=$1", photo.ID); err != nil {
		return photo, err
	}
	for _, tag := range tags {
		photo.Tags = append(photo.Tags, tag.Name)
	}

	photo.Permissions = &Permissions{
		photo.CanEdit(user),
		photo.CanDelete(user),
		photo.CanVote(user),
	}
	return photo, nil

}

func (mgr *defaultPhotoManager) ByOwnerID(page *Page, ownerID int64) (*PhotoList, error) {

	var (
		photos []Photo
		err    error
		total  int64
	)
	if ownerID == 0 {
		return nil, nil
	}
	if total, err = mgr.dbMap.SelectInt("SELECT COUNT(id) FROM photos WHERE owner_id=$1", ownerID); err != nil {
		return nil, err
	}

	if _, err = mgr.dbMap.Select(&photos,
		"SELECT * FROM photos WHERE owner_id = $1"+
			"ORDER BY (up_votes - down_votes) DESC, created_at DESC LIMIT $2 OFFSET $3",
		ownerID, page.Size, page.Offset); err != nil {
		return nil, err
	}
	return NewPhotoList(photos, total, page.Index), nil
}

func (mgr *defaultPhotoManager) Search(page *Page, q string) (*PhotoList, error) {

	var (
		clauses []string
		params  []interface{}
		err     error
		photos  []Photo
		total   int64
	)

	if q == "" {
		return nil, nil
	}

	for num, word := range strings.Split(q, " ") {
		word = strings.TrimSpace(word)
		if word == "" || num > 6 {
			break
		}

		num += 1

		if strings.HasPrefix(word, "@") {
			word = word[1:]
			clauses = append(clauses, fmt.Sprintf(
				"SELECT p.* FROM photos p "+
					"INNER JOIN users u ON u.id = p.owner_id  "+
					"WHERE UPPER(u.name::text) = UPPER($%d)", num))
		} else if strings.HasPrefix(word, "#") {
			word = word[1:]
			clauses = append(clauses, fmt.Sprintf(
				"SELECT p.* FROM photos p "+
					"INNER JOIN photo_tags pt ON pt.photo_id = p.id "+
					"INNER JOIN tags t ON pt.tag_id=t.id "+
					"WHERE UPPER(t.name::text) = UPPER($%d)", num))
		} else {
			word = "%" + word + "%"
			clauses = append(clauses, fmt.Sprintf(
				"SELECT DISTINCT p.* FROM photos p "+
					"INNER JOIN users u ON u.id = p.owner_id  "+
					"LEFT JOIN photo_tags pt ON pt.photo_id = p.id "+
					"LEFT JOIN tags t ON pt.tag_id=t.id "+
					"WHERE UPPER(p.title::text) LIKE UPPER($%d) OR "+
					"UPPER(u.name::text) LIKE UPPER($%d) OR t.name LIKE $%d",
				num, num, num))
		}

		params = append(params, interface{}(word))
	}

	clausesSql := strings.Join(clauses, " INTERSECT ")

	countSql := fmt.Sprintf("SELECT COUNT(id) FROM (%s) q", clausesSql)

	if total, err = mgr.dbMap.SelectInt(countSql, params...); err != nil {
		return nil, err
	}

	numParams := len(params)

	sql := fmt.Sprintf("SELECT * FROM (%s) q ORDER BY (up_votes - down_votes) DESC, created_at DESC LIMIT $%d OFFSET $%d",
		clausesSql, numParams+1, numParams+2)

	params = append(params, interface{}(page.Size))
	params = append(params, interface{}(page.Offset))

	if _, err = mgr.dbMap.Select(&photos, sql, params...); err != nil {
		return nil, err
	}
	return NewPhotoList(photos, total, page.Index), nil
}

func (mgr *defaultPhotoManager) All(page *Page, orderBy string) (*PhotoList, error) {

	var (
		total  int64
		photos []Photo
		err    error
	)
	if orderBy == "votes" {
		orderBy = "(up_votes - down_votes)"
	} else {
		orderBy = "created_at"
	}

	if total, err = mgr.dbMap.SelectInt("SELECT COUNT(id) FROM photos"); err != nil {
		return nil, err
	}

	if _, err = mgr.dbMap.Select(&photos,
		"SELECT * FROM photos "+
			"ORDER BY "+orderBy+" DESC LIMIT $1 OFFSET $2", page.Size, page.Offset); err != nil {
		return nil, err
	}
	return NewPhotoList(photos, total, page.Index), nil
}

func (mgr *defaultPhotoManager) GetTagCounts() ([]TagCount, error) {
	var tags []TagCount
	if _, err := mgr.dbMap.Select(&tags, "SELECT name, photo, num_photos FROM tag_counts"); err != nil {
		return tags, err
	}
	return tags, nil
}

type UserManager interface {
	Insert(user *User) error
	Update(user *User) error
	IsNameAvailable(user *User) (bool, error)
	IsEmailAvailable(user *User) (bool, error)
	GetActive(userID int64) (*User, error)
	GetByRecoveryCode(string) (*User, error)
	GetByEmail(string) (*User, error)
	Authenticate(identifier string, password string) (*User, error)
}

type defaultUserManager struct {
	dbMap *gorp.DbMap
}

func (mgr *defaultUserManager) Insert(user *User) error {
	return mgr.dbMap.Insert(user)
}

func (mgr *defaultUserManager) Update(user *User) error {
	_, err := mgr.dbMap.Update(user)
	return err
}

func (mgr *defaultUserManager) IsNameAvailable(user *User) (bool, error) {
	var (
		num int64
		err error
	)
	q := "SELECT COUNT(id) FROM users WHERE name=$1"
	if user.ID == 0 {
		num, err = mgr.dbMap.SelectInt(q, user.Name)
	} else {
		q += " AND id != $2"
		num, err = mgr.dbMap.SelectInt(q, user.Name, user.ID)
	}
	if err != nil {
		return false, err
	}
	return num == 0, nil
}

func (mgr *defaultUserManager) IsEmailAvailable(user *User) (bool, error) {
	var (
		num int64
		err error
	)
	q := "SELECT COUNT(id) FROM users WHERE email=$1"
	if user.ID == 0 {
		num, err = mgr.dbMap.SelectInt(q, user.Email)
	} else {
		q += " AND id != $2"
		num, err = mgr.dbMap.SelectInt(q, user.Email, user.ID)
	}
	if err != nil {
		return false, err
	}
	return num == 0, nil
}
func (mgr *defaultUserManager) GetActive(userID int64) (*User, error) {

	user := &User{}
	if err := mgr.dbMap.SelectOne(user, "SELECT * FROM users WHERE active=$1 AND id=$2", true, userID); err != nil {
		return user, err
	}
	return user, nil

}

func (mgr *defaultUserManager) GetByRecoveryCode(code string) (*User, error) {

	user := &User{}
	if code == "" {
		return user, sql.ErrNoRows
	}
	if err := mgr.dbMap.SelectOne(user, "SELECT * FROM users WHERE active=$1 AND recovery_code=$2", true, code); err != nil {
		return user, err
	}
	return user, nil

}
func (mgr *defaultUserManager) GetByEmail(email string) (*User, error) {
	user := &User{}
	if err := mgr.dbMap.SelectOne(user, "SELECT * FROM users WHERE active=$1 AND email=$2", true, email); err != nil {
		return user, err
	}
	return user, nil
}

func (mgr *defaultUserManager) Authenticate(identifier, password string) (*User, error) {
	user := &User{}

	if err := mgr.dbMap.SelectOne(user, "SELECT * FROM users WHERE active=$1 AND (email=$2 OR name=$2)", true, identifier); err != nil {
		if err == sql.ErrNoRows {
			return user, ErrInvalidLogin
		}
		return user, err
	}

	if !user.CheckPassword(password) {
		return user, ErrInvalidLogin
	}

	return user, nil
}

func NewUserManager(dbMap *gorp.DbMap) UserManager {
	return &defaultUserManager{dbMap}
}
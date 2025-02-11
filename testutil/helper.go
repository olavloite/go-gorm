package testutil

import (
	"fmt"
	"go/ast"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"database/sql/driver"
	"gorm.io/gorm"
	"gorm.io/gorm/utils"
)

type Config struct {
	Account   bool
	Pets      int
	Toys      int
	Company   bool
	Manager   bool
	Team      int
	Languages int
	Friends   int
	NamedPet  bool
}

func AssertObjEqual(t *testing.T, r, e interface{}, names ...string) {
	for _, name := range names {
		got := reflect.Indirect(reflect.ValueOf(r)).FieldByName(name).Interface()
		expect := reflect.Indirect(reflect.ValueOf(e)).FieldByName(name).Interface()
		t.Run(name, func(t *testing.T) {
			AssertEqual(t, got, expect)
		})
	}
}

func AssertEqual(t *testing.T, got, expect interface{}) {
	if !reflect.DeepEqual(got, expect) {
		isEqual := func() {
			if curTime, ok := got.(time.Time); ok {
				format := "2006-01-02T15:04:05Z07:00"

				if curTime.Round(time.Second).UTC().Format(format) != expect.(time.Time).Round(time.Second).UTC().Format(format) && curTime.Truncate(time.Second).UTC().Format(format) != expect.(time.Time).Truncate(time.Second).UTC().Format(format) {
					t.Errorf("%v: expect: %v, got %v after time round", utils.FileWithLineNum(), expect.(time.Time), curTime)
				}
			} else if fmt.Sprint(got) != fmt.Sprint(expect) {
				t.Errorf("%v: expect: %#v, got %#v", utils.FileWithLineNum(), expect, got)
			}
		}

		if fmt.Sprint(got) == fmt.Sprint(expect) {
			return
		}

		if reflect.Indirect(reflect.ValueOf(got)).IsValid() != reflect.Indirect(reflect.ValueOf(expect)).IsValid() {
			t.Errorf("%v: expect: %+v, got %+v", utils.FileWithLineNum(), expect, got)
			return
		}

		if valuer, ok := got.(driver.Valuer); ok {
			got, _ = valuer.Value()
		}

		if valuer, ok := expect.(driver.Valuer); ok {
			expect, _ = valuer.Value()
		}

		if got != nil {
			got = reflect.Indirect(reflect.ValueOf(got)).Interface()
		}

		if expect != nil {
			expect = reflect.Indirect(reflect.ValueOf(expect)).Interface()
		}

		if reflect.ValueOf(got).IsValid() != reflect.ValueOf(expect).IsValid() {
			t.Errorf("%v: expect: %+v, got %+v", utils.FileWithLineNum(), expect, got)
			return
		}

		if reflect.ValueOf(got).Kind() == reflect.Slice {
			if reflect.ValueOf(expect).Kind() == reflect.Slice {
				if reflect.ValueOf(got).Len() == reflect.ValueOf(expect).Len() {
					for i := 0; i < reflect.ValueOf(got).Len(); i++ {
						name := fmt.Sprintf(reflect.ValueOf(got).Type().Name()+" #%v", i)
						t.Run(name, func(t *testing.T) {
							AssertEqual(t, reflect.ValueOf(got).Index(i).Interface(), reflect.ValueOf(expect).Index(i).Interface())
						})
					}
				} else {
					name := reflect.ValueOf(got).Type().Elem().Name()
					t.Errorf("%v expects length: %v, got %v (expects: %+v, got %+v)", name, reflect.ValueOf(expect).Len(), reflect.ValueOf(got).Len(), expect, got)
				}
				return
			}
		}

		if reflect.ValueOf(got).Kind() == reflect.Struct {
			if reflect.ValueOf(expect).Kind() == reflect.Struct {
				if reflect.ValueOf(got).NumField() == reflect.ValueOf(expect).NumField() {
					exported := false
					for i := 0; i < reflect.ValueOf(got).NumField(); i++ {
						if fieldStruct := reflect.ValueOf(got).Type().Field(i); ast.IsExported(fieldStruct.Name) {
							exported = true
							field := reflect.ValueOf(got).Field(i)
							t.Run(fieldStruct.Name, func(t *testing.T) {
								AssertEqual(t, field.Interface(), reflect.ValueOf(expect).Field(i).Interface())
							})
						}
					}

					if exported {
						return
					}
				}
			}
		}

		if reflect.ValueOf(got).Type().ConvertibleTo(reflect.ValueOf(expect).Type()) {
			got = reflect.ValueOf(got).Convert(reflect.ValueOf(expect).Type()).Interface()
			isEqual()
		} else if reflect.ValueOf(expect).Type().ConvertibleTo(reflect.ValueOf(got).Type()) {
			expect = reflect.ValueOf(got).Convert(reflect.ValueOf(got).Type()).Interface()
			isEqual()
		} else {
			t.Errorf("%v: expect: %+v, got %+v", utils.FileWithLineNum(), expect, got)
			return
		}
	}
}

func GetUser(name, id string, config Config) *User {
	var (
		birthday = time.Now().Round(time.Second)
		user     = User{
			Name: name,
			Age:  18,
			Birthday: spanner.NullTime{
				Time:  birthday,
				Valid: true,
			},
		}
	)

	if config.Account {
		user.Account = Account{Number: name + "_account"}
	}

	for i := 0; i < config.Pets; i++ {
		user.Pets = append(user.Pets, &Pet{Name: name + "_pet_" + strconv.Itoa(i+1)})
	}

	for i := 0; i < config.Toys; i++ {
		user.Toys = append(user.Toys, Toy{Name: name + "_toy_" + strconv.Itoa(i+1)})
	}

	if config.Company {
		user.Company = Company{Name: "company-" + name}
	}

	if config.Manager {
		user.Manager = GetUser(name+"_manager", "", Config{})
	}

	for i := 0; i < config.Team; i++ {
		user.Team = append(user.Team, *GetUser(name+"_team_"+strconv.Itoa(i+1), "", Config{}))
	}

	for i := 0; i < config.Languages; i++ {
		name := name + "_locale_" + strconv.Itoa(i+1)
		language := Language{Code: name, Name: name}
		user.Languages = append(user.Languages, language)
	}

	for i := 0; i < config.Friends; i++ {
		user.Friends = append(user.Friends, GetUser(name+"_friend_"+strconv.Itoa(i+1), "", Config{}))
	}

	if config.NamedPet {
		user.NamedPet = &Pet{Name: name + "_namepet"}
	}

	return &user
}

func CheckPet(t *testing.T, db *gorm.DB, pet Pet, expect Pet) {
	if pet.ID != 0 {
		var newPet Pet
		if err := db.Where("id = ?", pet.ID).First(&newPet).Error; err != nil {
			t.Fatalf("errors happened when query: %v", err)
		} else {
			AssertObjEqual(t, newPet, pet, "ID", "CreatedAt", "UpdatedAt", "UserID", "Name")
			AssertObjEqual(t, newPet, expect, "ID", "CreatedAt", "UpdatedAt", "UserID", "Name")
		}
	}

	AssertObjEqual(t, pet, expect, "ID", "CreatedAt", "UpdatedAt", "UserID", "Name")

	AssertObjEqual(t, pet.Toy, expect.Toy, "ID", "CreatedAt", "UpdatedAt", "Name", "OwnerID", "OwnerType")

	if expect.Toy.Name != "" && expect.Toy.OwnerType != "pets" {
		t.Errorf("toys's OwnerType, expect: %v, got %v", "pets", expect.Toy.OwnerType)
	}
}

func CheckUser(t *testing.T, db *gorm.DB, user User, expect User) {
	if user.ID != 0 {
		var newUser User
		if err := db.Where("id = ?", user.ID).First(&newUser).Error; err != nil {
			t.Fatalf("errors happened when query: %v", err)
		} else {
			AssertObjEqual(t, newUser, user, "ID", "CreatedAt", "UpdatedAt", "Name", "Age", "Birthday", "CompanyID", "ManagerID", "Active")
		}
	}

	AssertObjEqual(t, user, expect, "ID", "CreatedAt", "UpdatedAt", "Name", "Age", "Birthday", "CompanyID", "ManagerID", "Active")

	t.Run("Account", func(t *testing.T) {
		AssertObjEqual(t, user.Account, expect.Account, "ID", "CreatedAt", "UpdatedAt", "UserID", "Number")

		if user.Account.Number != "" {
			if !user.Account.UserID.Valid {
				t.Errorf("Account's foreign key should be saved")
			} else {
				var account Account
				db.First(&account, "user_id = ?", user.ID)
				AssertObjEqual(t, account, user.Account, "ID", "CreatedAt", "UpdatedAt", "UserID", "Number")
			}
		}
	})

	t.Run("Pets", func(t *testing.T) {
		if len(user.Pets) != len(expect.Pets) {
			t.Fatalf("pets should equal, expect: %v, got %v", len(expect.Pets), len(user.Pets))
		}

		sort.Slice(user.Pets, func(i, j int) bool {
			return user.Pets[i].ID > user.Pets[j].ID
		})

		sort.Slice(expect.Pets, func(i, j int) bool {
			return expect.Pets[i].ID > expect.Pets[j].ID
		})

		for idx, pet := range user.Pets {
			if pet == nil || expect.Pets[idx] == nil {
				t.Errorf("pets#%v should equal, expect: %v, got %v", idx, expect.Pets[idx], pet)
			} else {
				CheckPet(t, db, *pet, *expect.Pets[idx])
			}
		}
	})

	t.Run("Toys", func(t *testing.T) {
		if len(user.Toys) != len(expect.Toys) {
			t.Fatalf("toys should equal, expect: %v, got %v", len(expect.Toys), len(user.Toys))
		}

		sort.Slice(user.Toys, func(i, j int) bool {
			return user.Toys[i].ID > user.Toys[j].ID
		})

		sort.Slice(expect.Toys, func(i, j int) bool {
			return expect.Toys[i].ID > expect.Toys[j].ID
		})

		for idx, toy := range user.Toys {
			if toy.OwnerType != "users" {
				t.Errorf("toys's OwnerType, expect: %v, got %v", "users", toy.OwnerType)
			}

			AssertObjEqual(t, toy, expect.Toys[idx], "ID", "CreatedAt", "UpdatedAt", "Name", "OwnerID", "OwnerType")
		}
	})

	t.Run("Company", func(t *testing.T) {
		AssertObjEqual(t, user.Company, expect.Company, "ID", "Name")
	})

	t.Run("Manager", func(t *testing.T) {
		if user.Manager != nil {
			if user.ManagerID.IsNull() {
				t.Errorf("Manager's foreign key should be saved")
			} else {
				var manager User
				db.First(&manager, "id = ?", user.ManagerID.StringVal)
				AssertObjEqual(t, manager, user.Manager, "ID", "CreatedAt", "UpdatedAt", "Name", "Age", "Birthday", "CompanyID", "ManagerID", "Active")
				AssertObjEqual(t, manager, expect.Manager, "ID", "CreatedAt", "UpdatedAt", "Name", "Age", "Birthday", "CompanyID", "ManagerID", "Active")
			}
		} else if !user.ManagerID.IsNull() {
			t.Errorf("Manager should not be created for zero value, got: %+v", user.ManagerID)
		}
	})

	t.Run("Team", func(t *testing.T) {
		if len(user.Team) != len(expect.Team) {
			t.Fatalf("Team should equal, expect: %v, got %v", len(expect.Team), len(user.Team))
		}

		sort.Slice(user.Team, func(i, j int) bool {
			return user.Team[i].ID > user.Team[j].ID
		})

		sort.Slice(expect.Team, func(i, j int) bool {
			return expect.Team[i].ID > expect.Team[j].ID
		})

		for idx, team := range user.Team {
			AssertObjEqual(t, team, expect.Team[idx], "ID", "CreatedAt", "UpdatedAt", "Name", "Age", "Birthday", "CompanyID", "ManagerID", "Active")
		}
	})

	t.Run("Languages", func(t *testing.T) {
		if len(user.Languages) != len(expect.Languages) {
			t.Fatalf("Languages should equal, expect: %v, got %v", len(expect.Languages), len(user.Languages))
		}

		sort.Slice(user.Languages, func(i, j int) bool {
			return strings.Compare(user.Languages[i].Code, user.Languages[j].Code) > 0
		})

		sort.Slice(expect.Languages, func(i, j int) bool {
			return strings.Compare(expect.Languages[i].Code, expect.Languages[j].Code) > 0
		})
		for idx, language := range user.Languages {
			AssertObjEqual(t, language, expect.Languages[idx], "Code", "Name")
		}
	})

	t.Run("Friends", func(t *testing.T) {
		if len(user.Friends) != len(expect.Friends) {
			t.Fatalf("Friends should equal, expect: %v, got %v", len(expect.Friends), len(user.Friends))
		}

		sort.Slice(user.Friends, func(i, j int) bool {
			return user.Friends[i].ID > user.Friends[j].ID
		})

		sort.Slice(expect.Friends, func(i, j int) bool {
			return expect.Friends[i].ID > expect.Friends[j].ID
		})

		for idx, friend := range user.Friends {
			AssertObjEqual(t, friend, expect.Friends[idx], "ID", "CreatedAt", "UpdatedAt", "Name", "Age", "Birthday", "CompanyID", "ManagerID", "Active")
		}
	})
}

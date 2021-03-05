package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

type Store struct {
	Scripts     map[string]string `json:"scripts"`
	scriptsLock sync.Mutex
}

func NewStore() *Store {
	s := new(Store)
	s.Scripts = make(map[string]string)

	bytes, err := ioutil.ReadFile("store.json")
	if err != nil {
		logger.Println(err)
	}
	err = json.Unmarshal(bytes, &s)
	if err != nil {
		logger.Println(err)
	}

	return s
}

func (s *Store) save() {
	f, err := os.Create("store.json")
	if err != nil {
		logger.Println(err)
		return
	}
	defer f.Close()

	data, err := json.Marshal(s)
	if err != nil {
		logger.Println(err)
		return
	}
	f.Write(data)
}

func (s *Store) SetScript(guildID, name, url string) {
	s.scriptsLock.Lock()
	s.Scripts[guildID+"-"+name] = url
	s.scriptsLock.Unlock()

	s.save()
}

func (s *Store) GetScript(guildID, name string) string {
	s.scriptsLock.Lock()
	defer s.scriptsLock.Unlock()
	return s.Scripts[guildID+"-"+name]
}

func (s *Store) RemoveScript(guildID, name string) {
	s.scriptsLock.Lock()
	defer s.scriptsLock.Unlock()
	delete(s.Scripts, guildID+"-"+name)

	s.save()
}

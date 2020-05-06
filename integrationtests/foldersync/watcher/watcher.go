package watcher

import (
	"sync"

	"github.com/fsnotify/fsnotify"
	logging "github.com/ipfs/go-log"
)

var (
	log = logging.Logger("watcher")
)

type Handler func(fileName string) error

type FolderWatcher struct {
	w        *fsnotify.Watcher
	onCreate Handler

	stopWatch chan struct{}
	done      chan struct{}

	lock    sync.Mutex
	started bool
	closed  bool
}

func New(path string, onCreate Handler) (*FolderWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	if err = watcher.Add(path); err != nil {
		return nil, err
	}

	return &FolderWatcher{
		w:         watcher,
		onCreate:  onCreate,
		stopWatch: make(chan struct{}),
		done:      make(chan struct{}),
	}, nil
}

func (fw *FolderWatcher) Close() {
	fw.lock.Lock()
	defer fw.lock.Unlock()

	if !fw.started || fw.closed {
		return
	}
	fw.closed = true

	close(fw.stopWatch)
	<-fw.done
	_ = fw.w.Close()
}

func (fw *FolderWatcher) Watch() {
	fw.lock.Lock()
	defer fw.lock.Unlock()
	if fw.started {
		return
	}

	fw.started = true
	go func() {
		for {
			select {
			case <-fw.stopWatch:
				log.Info("grafceful shutdown")
				close(fw.done)
				return
			case event, ok := <-fw.w.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					log.Debug("created file:", event.Name)

					if err := fw.onCreate(event.Name); err != nil {
						log.Errorf("error when calling onCreate for %s", event.Name)
					}
				}
			case err, ok := <-fw.w.Errors:
				if !ok {
					return
				}
				log.Error(err)
			}
		}
	}()
}

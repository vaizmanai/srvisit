package contact

import (
	"../../common"
	"sync"
)

//тип для контакта
type Contact struct {
	Id      int
	Caption string
	Type    string //cont - контакт, fold - папка
	Pid     string
	Digest  string //но тут digest
	Salt    string

	Inner *Contact
	Next  *Contact

	mutex sync.RWMutex
}

func delContact(first *Contact, id int) *Contact {
	if first == nil {
		return first
	}

	for first != nil && first.Id == id {
		first = first.Next
	}

	res := first

	for first != nil {
		for first.Next != nil && first.Next.Id == id {
			first.Next = first.Next.Next
		}

		if first.Inner != nil {
			first.Inner = delContact(first.Inner, id)
		}

		first = first.Next
	}

	return res
}

func DelContact(first *Contact, id int) *Contact {
	if first != nil {
		first.mutex.Lock()
		defer first.mutex.Unlock()
	}
	return delContact(first, id)
}

func getContact(first *Contact, id int) *Contact {

	for first != nil {
		if first.Id == id {
			return first
		}

		if first.Inner != nil {
			inner := getContact(first.Inner, id)
			if inner != nil {
				return inner
			}
		}

		first = first.Next
	}

	return nil
}

func GetContact(first *Contact, id int) *Contact {
	if first != nil {
		first.mutex.RLock()
		defer first.mutex.RUnlock()
	}
	return getContact(first, id)
}

func getContactByPid(first *Contact, pid string) *Contact {
	for first != nil {
		if common.CleanPid(first.Pid) == pid {
			return first
		}

		if first.Inner != nil {
			inner := getContactByPid(first.Inner, pid)
			if inner != nil {
				return inner
			}
		}

		first = first.Next
	}

	return nil
}

func GetContactByPid(first *Contact, pid string) *Contact {
	if first != nil {
		first.mutex.RLock()
		defer first.mutex.RUnlock()
	}
	return getContactByPid(first, pid)
}

func getNewId(contact *Contact) int {
	if contact == nil {
		return 1
	}

	r := 1

	for contact != nil {

		if contact.Id >= r {
			r = contact.Id + 1
		}

		if contact.Inner != nil {
			t := getNewId(contact.Inner)
			if t >= r {
				r = t + 1
			}
		}

		contact = contact.Next
	}

	return r
}

func GetNewId(contact *Contact) int {
	if contact != nil {
		contact.mutex.RLock()
		defer contact.mutex.RUnlock()
	}
	return getNewId(contact)
}

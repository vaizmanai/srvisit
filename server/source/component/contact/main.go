package contact

import "../../common"

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
}

func DelContact(first *Contact, id int) *Contact {
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
			first.Inner = DelContact(first.Inner, id)
		}

		first = first.Next
	}

	return res
}

func GetContact(first *Contact, id int) *Contact {

	for first != nil {
		if first.Id == id {
			return first
		}

		if first.Inner != nil {
			inner := GetContact(first.Inner, id)
			if inner != nil {
				return inner
			}
		}

		first = first.Next
	}

	return nil
}

func GetContactByPid(first *Contact, pid string) *Contact {

	for first != nil {
		if common.CleanPid(first.Pid) == pid {
			return first
		}

		if first.Inner != nil {
			inner := GetContactByPid(first.Inner, pid)
			if inner != nil {
				return inner
			}
		}

		first = first.Next
	}

	return nil
}

func GetNewId(contact *Contact) int {
	if contact == nil {
		return 1
	}

	r := 1

	for contact != nil {

		if contact.Id >= r {
			r = contact.Id + 1
		}

		if contact.Inner != nil {
			t := GetNewId(contact.Inner)
			if t >= r {
				r = t + 1
			}
		}

		contact = contact.Next
	}

	return r
}

package textwords

import "testing"

func TestContainsAnyWord(t *testing.T) {
	if !ContainsAnyWord("у меня понос", []string{"понос"}) {
		t.Fatal()
	}
	if ContainsAnyWord("переработал", []string{"работа"}) {
		t.Fatal("whole word таблицы: без подстроки работа inside переработал")
	}
}

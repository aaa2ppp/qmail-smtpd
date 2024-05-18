package main

/*
unsigned int scan_ulong(s,u) register char *s; register unsigned long *u;
{
  register unsigned int pos; register unsigned long result;
  register unsigned long c;
  pos = 0; result = 0;
  while ((c = (unsigned long) (unsigned char) (s[pos] - '0')) < 10)
    { result = result * 10 + c; ++pos; }
  *u = result; return pos;
}
*/

func scan_ulong(s string) (int, uint) {
	var pos int
	var result uint
	for {
		c := uint(s[pos] - '0')
		if !( c < 10) {
			break
		}
		result = result * 10 + c
		pos++ 
	}
	return pos, result
}

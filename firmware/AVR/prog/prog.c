#define TTYNAME		"/dev/cuaU1"
#if 0
#define TTYNAME		"/dev/ttyS0"
#define TTYNAME		"/tmp/ttyDUMMY"
#define TTYNAME		"/dev/cuaU0"
#define TTYNAME		"/dev/cua00"
#define TTYNAME		"/dev/ttyUSB0"
#endif

#include <stdio.h>		/* Standard input/output */
#include <string.h>		/* String function */
#include <unistd.h>		/* UNIX standard functions */
#include <fcntl.h>		/* File control */
#include <termios.h>		/* POSIX terminal control */
#include <err.h>
#include <stdlib.h>

int fd;				/* descriptor for RS232 interface file */


char
uart_getchar(void)
{	
	char r;

	if (read(fd, &r, 1) != 1)
		errx(1, "com: read error");
	return r;
}


void
uart_putchar(char s)
{
	char r;

	if (write(fd, &s, 1) != 1)
		errx(1, "com: write error");
	r = uart_getchar();
	if(s != r)
		errx(1, "sent: 0x%02hhx != read: 0x%02hhx", s, r);
}


void
initRS232()
{
	struct termios tty;
	char ttyName[] = TTYNAME;

	fd = open(ttyName, O_RDWR | O_NOCTTY | O_NDELAY);
	if (fd == -1)
		err(1, "initRS232: Unable to open %s", ttyName);
	fcntl(fd, F_SETFL, 0);
	if (tcgetattr(fd, &tty))
		err(1, "initRS232: error from tcgetattr");
	tty.c_cflag &= ~PARENB;	/* no parity */
	tty.c_cflag &= ~CSTOPB;	/* 1 stop bit */
	tty.c_cflag &= ~CSIZE;	
	tty.c_cflag |= CS8;	/* 8 bits */
	tty.c_cflag &= ~CRTSCTS;/* no harware ctrl */
	tty.c_cflag |= (CLOCAL | CREAD);	/* enable read */
	tty.c_lflag &= ~ICANON;	/* non-canonical mode */
	tty.c_lflag &= ~ISIG;	/* disable interpretation of */
	/* INTR, QUIT and SUSP */
	tty.c_lflag &= ~(ECHO | ECHONL | ECHOE); /* disable echo, erasure */
	tty.c_iflag &= ~(IXON | IXOFF | IXANY | ICRNL | INLCR);
	/* prevent special interpretation of */
	/* output bytes (e.g. newline chars) */
	tty.c_oflag &= ~OPOST;
	/* wait for up to 7s (70 deciseconds), */
	/* returning as soon as any data is received */
	cfmakeraw(&tty);  //seems important (it adds sth relevant not
			  //to translate some 'binary' characters)
	tty.c_cc[VTIME] = 70;
	tty.c_cc[VMIN] = 0;
	cfsetispeed(&tty, B9600);	/* in speed */
	cfsetospeed(&tty, B9600);	/* out speed */

	if (tcsetattr(fd, TCSANOW, &tty))
		err(1, "initRS232: error from tcsetattr");

	tcflush(fd, TCIOFLUSH);
}


struct hexRec {
	char n, a1, a2, rt;
	char d[255];
	char cs;
};


char
readB(FILE *f)
{
	char c[3];
	char b;

	if((c[0] = getc(f)) == EOF)
		errx(1, "reading a record from .hex failed unexpectedly");
	if((c[1] = getc(f)) == EOF)
		errx(1, "reading a record from .hex failed unexpectedly");
	c[2] = 0;
	sscanf(c, "%hhx", &b);
	return b;
}


void
readRecord(FILE *f, struct hexRec *r)
{
	int z;
	int i;
	char c, s;
 
	while((z = getc(f)) != ':')
		if(z == EOF)
			errx(1, "EOF reached in .hex while hex-EOF not seen");
	s = r->n = readB(f);
	s += r->a1 = readB(f);
	s += r->a2 = readB(f);
	s += r->rt = readB(f);
	for(i = 0; i < r->n; i++) {
		c = readB(f);
		r->d[i] = c;
		s += c;
	}
	s += r->cs = readB(f);

	if(s)
		errx(1, "hex checksum problem");
}


void
sync(void)
{
	usleep(50000);
	tcflush(fd, TCIOFLUSH);
	usleep(50000);
	do 
		uart_putchar('S');
	while(uart_getchar() != 'Y');
}

void
setWAddr(int w)
{
	char c;

	uart_putchar('A');
	uart_putchar(w >> 8);	/* high byte of the word-address */	
	uart_putchar(w); /* low */
	if((c = uart_getchar()) != '\r')
		errx(1, "wrong response to set word-address"
		  " %x: %hhx\n", w, c);
}


void
writePage(int wp) {
	char c;

	if(wp < 0)
		return;

	setWAddr(wp);
	printf("writing word-page %04x\n", wp);
	uart_putchar('M');
	if((c = uart_getchar()) != '\r')
		errx(1, "wrong response to write page "
		  "%04x: %hhx\n", wp, c);
}


int
main(int argc, char *argv[])
{
	FILE *f;
	struct hexRec rr;
	struct hexRec *r = &rr;
	int i;
	int w, ww, wp;
	char c;
	char name[] = "RS";
	char *pc;
	char defaultHexName[] = "app.hex";
	char *pHexName;

	warnx("NB: If you ever run into an issue, verify with a more\n"
	  "regular programmer, this simple one may be the culprit.\n");

	initRS232();

	/* check that we have a reasonable partner */
	for(;;) {
		sync();
		uart_putchar('I');
		pc = &name[0];
		while((c = uart_getchar()) == *pc) 
			if(*pc++ == 0)
				goto IOK;
		warnx("no reasonable partner, waiting...");
	}
IOK:
	printf("I_OK\n");

	/* erase the chip */
	uart_putchar('E');
	if((c = uart_getchar()) != '\r')
		errx(1, "wrong response to request to erase: %hhx\n", c);
	printf("ERASE_OK\n");

	/* program hex file */
	if(argc == 1) {
		pHexName = defaultHexName;
		warnx("Hex-file not specified, trying '%s'", pHexName);
	} else 
		pHexName = argv[1];
	if((f = fopen(pHexName, "r")) == NULL)
		errx(1, "cannot open hex-file '%s'", pHexName);
	printf("programming %s\n", pHexName);


	wp = -1;	/* working page: no unprogrammed data exists */
	for(;;) {
		readRecord(f, r);
		if(r->rt == 1) {	/* end of the hex file */
			printf("End of .hex reached\n");
			writePage(wp);
			break;
		} else if(r->rt != 0)	/* not a data record */
			errx(1, "encountered unknown record type in hex-file, "
			  "we only support I8HEX, with types 0 or 1");

		/* ignore data records with no data (if ever appear) */
		if(r->n == 0)
			continue;
		
		/* word-address of the 1st data of the record */
		w = ((unsigned char) r->a1 << 8 | (unsigned char) r->a2) >> 1;
	
		printf("record starting byte-address: %02hhx %02hhx, "
		  "ie, word-address: %04x\n", r->a1, r->a2, w);

		/* 
		 * process the record 
		 * NB avr uses little-endian format, and low byte comes
		 * first in the hex file
		 */
		for(i = 0; i < r->n; ) {
			ww = w & 0xffc0;
			if(ww > wp) {
				writePage(wp);
				wp = ww;
				setWAddr(w);
			} else if(ww < wp)
				errx(1, "we only handle hex files with "
				  "monotonically increasing addresses, sorry");
			else if(i == 0)
				setWAddr(w);
			uart_putchar('C');
			uart_putchar(r->d[i++]);	/* low byte */
			uart_putchar(r->d[i++]);	/* high byte */
			if((c = uart_getchar()) != '\r')
				errx(1, "wrong response to write data at"
				  "word-address %x: %hhx\n", w, c);
			w++;
		}
	}

	printf("Exiting the programmer.\n");
	uart_putchar('X');
	if((c = uart_getchar()) != '\r')
			errx(1, "wrong response to exit programming\n");
	return 0;
}


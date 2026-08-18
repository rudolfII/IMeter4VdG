#include <stdlib.h>
#include <string.h>
#include <avr/io.h>
#include <avr/interrupt.h>
#include <avr/wdt.h>
#include <util/setbaud.h>	/* sets UBRR._VALUEs */

#define COMM_LEN	15

/* We use an interrupt-driven cyclic receive buffer */
#define RX_BUF_SIZE	10
volatile char rxbuf[RX_BUF_SIZE];
volatile int rxbn; /* number of available characters */
volatile int rxbi; /* index of first unread-character */

ISR(USART_RX_vect)
{
	int i;

	if(rxbn < RX_BUF_SIZE) {
		i = rxbi + rxbn;
		if (i >= RX_BUF_SIZE)
			i = i - RX_BUF_SIZE;
		rxbuf[i] = UDR0;
		rxbn++;
	}
}


static void
uart_init(void) {
	UBRR0H = UBRRH_VALUE;
	UBRR0L = UBRRL_VALUE;
	#if USE_2X
		UCSR0A |= _BV(U2X0);
	#else
		UCSR0A &= ~(_BV(U2X0));
	#endif
	UCSR0C = _BV(UCSZ01) | _BV(UCSZ00); /* 8-bit data */
	/* Enable RX and TX, interrupt on reception */
	UCSR0B = _BV(RXEN0) | _BV(TXEN0) | _BV(RXCIE0);
}


static int
uart_getc()
{
	char r;

	if (rxbn > 0) {
		cli();
		r = rxbuf[rxbi];
		rxbi++;
		if(rxbi == RX_BUF_SIZE)
			rxbi = 0;
		rxbn--;
		sei();
		return r;
	} else
		return -1;
}


static void
uart_putc(char c)
{
	loop_until_bit_is_set(UCSR0A, UDRE0);
	UDR0 = c;
}


static void
uart_outs(char *s)
{
	char c;

	while((c = *s++))
		uart_putc(c);
}


__attribute__((OS_main))
int
main(void)
{ 
	int c;
	char comm[COMM_LEN + 1];
	int commPos = 0;
	//long val;
	char *valStr;

	MCUSR = 0;
	wdt_disable();

	rxbn = 0;
	rxbi = 0;
	uart_init();
	sei();

	uart_outs("application starting...\n\r");

	for(;;) {
		c = uart_getc();
		if (c != -1) {
		// OR, to read all the available characters at once:
		// while((c = uart_getc()) != -1) {
			if (c != '\r') {
				if (commPos < COMM_LEN)
					comm[commPos++] = c;
			} else {
				comm[commPos] = '\0';
				commPos = 0;

				strtok(comm, " ");
				valStr = strtok(NULL, " ");
				if(valStr == NULL)
					valStr = "";
				//val = strtol(valStr, NULL, 0);

				if (!strcmp(comm, "test"))
					uart_outs("test OK\n\r");
				else if (!strcmp(comm, "reset")) {
					asm ("cli");
					MCUSR = 0;
					asm volatile ("jmp 0x7c00");
				} else {
					uart_outs(comm);
					uart_putc(' ');
					uart_outs(valStr);
					uart_outs("\n\r");
				}
			}
		}
	}
}


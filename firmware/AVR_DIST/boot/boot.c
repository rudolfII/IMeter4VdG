#include <avr/io.h>
#include <avr/wdt.h>

#ifndef BAUD
	#warning 'BAUD not defined, defaults to 9600'
	#define BAUD	9600
#endif
#ifndef F_CPU
	#warning 'F_CPU not defined, defaults to 16000000'
	#define F_CPU	16000000
#endif
#include <util/setbaud.h>	/* sets UBRR._VALUEs */


#define	BOOT_START	0x7c00	// byte address
/* atmega328P */		
#define	PAGESIZE	128	// bytes, ie 64 words
		

static void
uart_init(void) {
	UBRR0H = UBRRH_VALUE;
	UBRR0L = UBRRL_VALUE;
	#if USE_2X
		UCSR0A |= _BV(U2X0);
	#else
		UCSR0A &= ~(_BV(U2X0));
	#endif
	/* set 8b, 1stop, no parity */
	UCSR0C = _BV(UCSZ01) | _BV(UCSZ00);
	/* enable RX and TX */
	UCSR0B = _BV(RXEN0) | _BV(TXEN0);
}


static void
uart_putc(char c)
{
	while(!(UCSR0A & _BV(UDRE0)))
		;
	UDR0 = c;
	while(!(UCSR0A & _BV(UDRE0)))
		;
}


static unsigned char
uart_getc(void)
{
	char c;

	while(!(UCSR0A & _BV(RXC0)))
		;
	c = UDR0;
	uart_putc(c);
	return c;
}


#include <avr/boot.h>
#include <avr/pgmspace.h>

#define _ENABLE_RWW_SECTION() boot_rww_enable()
#define _WAIT_FOR_SPM() boot_spm_busy_wait()
#define _LOAD_PROGRAM_MEMORY(addr) pgm_read_byte_near(addr)
#define _FILL_TEMP_WORD(addr,data) boot_page_fill(addr, data)
#define _PAGE_ERASE(addr) boot_page_erase(addr)
#define _PAGE_WRITE(addr) boot_page_write(addr)


#pragma GCC diagnostic ignored "-Wmaybe-uninitialized"
__attribute__ ((section (".init8"))) 
__attribute__ ((used)) 
int
main(void) 
{
	int addr;
	int w;
	char c;

	/*
	 * c-compiler expects r1 to be zero! 
	 * e.g., assigning zero to a variable is
	 * done blindly by using this r1 register
	 */
	asm("clr r1 ");

	/* If the reset was due to WDT timeout, jump to
	 * the application. Otherwise run the bootloader.
	 */ 
	if((MCUSR & _BV(WDRF)) != 0) {
		MCUSR = 0;
		wdt_disable();
		asm volatile ("jmp 0"); // run the application
	}

	wdt_enable(WDTO_8S);
	uart_init(); 
	uart_putc('-');	/* for diagnostics */
	for(;;) {
		c = uart_getc();
		wdt_reset();

		if(c == 'A') {	/* set address 
				 * flash addresses are given in words, not bytes */
			addr = uart_getc() << 8;	/* high byte */
			addr |= uart_getc();		/* low byte */
			uart_putc('\r');
		} else if(c == 'E') {	/* chip erase */
			for(addr = 0; addr < BOOT_START; addr += PAGESIZE) {
				/* byte-address, not word-address */
				_WAIT_FOR_SPM();  
				_PAGE_ERASE(addr);
			}
			uart_putc('\r');
		} else if(c == 'R') {	/* read program memory 
					 * send high, then low byte, of flash word */
			_WAIT_FOR_SPM();		
			_ENABLE_RWW_SECTION();
			uart_putc(_LOAD_PROGRAM_MEMORY((addr << 1) + 1 ));
			uart_putc(_LOAD_PROGRAM_MEMORY((addr << 1)));
			addr++; 
		} else if(c == 'S') {	/* sync */
			uart_putc('Y');
		} else if(c == 'C') {	/* write program memory */
			w = uart_getc();	/* low byte */
			w |= uart_getc() << 8;	/* high byte */
			_WAIT_FOR_SPM();
			_FILL_TEMP_WORD(addr << 1, w);
			addr++;
			uart_putc('\r');
		} else if(c == 'M') {	/* write page */ 
			_WAIT_FOR_SPM();		
			_PAGE_WRITE(addr << 1);
			uart_putc('\r');
		} else if(c == 'X') {	/* exit bootloader */
			_WAIT_FOR_SPM();		
			_ENABLE_RWW_SECTION();
			uart_putc('\r');
			/* wait for watchdog reset */
			while(1)
				uart_putc('z');	/* for diagnostics */
		} else if(c == 'I') {	/* return identifier */
			uart_putc('R'); 
			uart_putc('S'); 
			uart_putc(0);
		} else
			uart_putc('?');
	} // for(;;)

	return 0; // will never happen
}


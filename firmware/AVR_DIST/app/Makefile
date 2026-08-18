BAUD=9600
MCU=atmega328p
F_CPU=16000000U # 16 Mhz


CFLAGS=-g -DF_CPU=$(F_CPU) -mmcu=$(MCU) -Wall -Wextra \
	-DBAUD=$(BAUD) -Wa,-adhlns=app.lst -O0

all: app.hex

.SECONDARY: $(%.o)	#forbids removal of temporary targets

%.o: %.c
	avr-gcc $(CFLAGS) -c $< -o $@

%.elf: %.o
	avr-gcc $(CFLAGS) -o $@ $^

%.hex: %.elf
	avr-objcopy -j .text -j .data -O ihex $^ $@

#in case we ever program using usbasp; -D == do not erase the
#chip (to not erase the bootloader)
flash: app.hex
	avrdude -p$(MCU) -cusbasp -D -v -U flash:w:app.hex

lss: app.elf
	avr-objdump -j .text -j .data -h -S app.elf > app.lss

clean:
	rm -f *.o *.elf *.lst *.lss *.hex

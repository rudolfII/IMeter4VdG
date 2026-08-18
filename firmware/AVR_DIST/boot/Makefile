boot.hex: boot.elf
	avr-objcopy -j .text -j .data -O ihex boot.elf boot.hex

boot.o: boot.c
	avr-gcc -DBAUD=9600 -DF_CPU=16000000 -c -mmcu=atmega328p -Wall \
	  -o boot.o boot.c
# -mmcu needed to define instruction set

# the address like 0x7c00 below positions the bootloader
# and must agree with the programmed fuses
boot.elf: boot.o
	avr-gcc -mmcu=atmega328p \
	  -Wl,--section-start=.text=0x7c00 \
	  -nostartfiles \
	  -o boot.elf boot.o

# NB flashing the bootloader erases all other code
# ie, application has to be loaded afterwards
flash: boot.hex
	avrdude -p atmega328p -c usbasp -U flash:w:boot.hex:i

read:
	avrdude -p atmega328p -c usbasp -U flash:r:read.hex:i

readFuses:
	avrdude -p atmega328p -c usbasp \
	  -U hfuse:r:-:h -U lfuse:r:-:h -U efuse:r:-:h

flashFuses:
	avrdude -p atmega328p -c usbasp -v \
	  -U hfuse:w:0xDC:m -U lfuse:w:0xD7:m -U efuse:w:0xFC:m

clean:
	rm -f *.o *.elf *.hex *.lst

lst:
	avr-objdump -j .text -j .data -h -S boot.elf > boot.lst


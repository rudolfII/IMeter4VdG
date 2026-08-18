#include <TimerOne.h>

#define BAUD_SPEED  9600
#define COMM_LEN  15            // max. délka sériového příkazu
int ttime;
const byte interruptPin = 2;
const byte GatePin = 4;

// Proměnné používané v přerušení MUSÍ být volatile
volatile unsigned long pocet;
volatile unsigned long proud;
volatile unsigned long integral;
unsigned long chargemax;
volatile boolean stop_count;

// BEZPEČNÁ POLE SE SPRÁVNOU VELIKOSTÍ PRO "I" A "Q"
char buffer1[32];
char buffer2[32];
String firmware = "FW 8.2";

//String id1 = "CM 001";
String id1 = "ID001A";
char comm[COMM_LEN + 1];
int commPos = 0;

// Příznaky pro komunikaci mezi přerušením a hlavní smyčkou loop()
volatile boolean dataReady = false;
volatile boolean triggerTOff = false;
void (*resetFunc)(void) = 0;
void
setup(void)
{
    integral = 0;
    pocet = 0;
    ttime = 1;
    chargemax = 100;
    stop_count = true;
    dataReady = false;
    triggerTOff = false;
    Serial.begin(BAUD_SPEED);

    // Vyčištění paměťového registru sériové linky Arduina po startu
    while (Serial.available() > 0) {
        Serial.read();
    }
    commPos = 0;
    comm[0] = '\0';
    pinMode(GatePin, OUTPUT);
    digitalWrite(GatePin, HIGH);        // Výchozí stav (Hradlo vypnuté)
    pinMode(interruptPin, INPUT_PULLUP);
}

void
loop(void)
{

    // 1. BEZPEČNÝ VÝPIS NAMĚŘENÝCH DAT
    if (dataReady) {
        dataReady = false;      // Reset příznaku

        // Vytiskneme data, která časovač v přesný moment připravil do bufferů
        Serial.println(buffer1);        // Výpis proudu I2
        Serial.println(buffer2);        // Výpis náboje Q2
    }

    // Pokud limit náboje přetekl uvnitř časovače, odešleme TOff do PC zde v loop()
    if (triggerTOff) {
        triggerTOff = false;
        Serial.println("TOff");
    }

    // 2. ČTENÍ SÉRIOVÉ LINKY BEZ BLOKOVÁNÍ
    while (Serial.available() > 0) {
        char c = Serial.read();
        if (c != '\r' && c != '\n') {
            if (commPos < COMM_LEN) {
                comm[commPos++] = c;
            }
        } else if (commPos > 0) {       // Zpracuj příkaz pouze pokud byl nějaký text přijat
            comm[commPos] = '\0';
            commPos = 0;        // Reset indexu pro další příkaz

            // Bezpečné parsování příkazu a hodnoty
            char *cmdStr = strtok(comm, " ");
            char *valStr = strtok(NULL, " ");
            long val = 0;
            if (valStr != NULL) {
                val = strtol(valStr, NULL, 10);
            }
            if (cmdStr != NULL) {
                if (!strcmp(cmdStr, "START")) {
                    noInterrupts();     // Bezpečné vynulování paměti před startem hardware
                    pocet = 0;
                    integral = 0;
                    dataReady = false;
                    triggerTOff = false;
                    interrupts();
                    attachInterrupt(digitalPinToInterrupt(interruptPin),
                                    pocitadlo, FALLING);
                    Timer1.initialize(1000000UL * ttime);
                    Timer1.attachInterrupt(casovac);
                }

                else if (!strcmp(cmdStr, "STOP")) {
                    Timer1.stop();
                    Timer1.detachInterrupt();
                    detachInterrupt(digitalPinToInterrupt(interruptPin));
                    dataReady = false;
                }

                else if (!strcmp(cmdStr, "QMAX")) {
                    chargemax = val;
                }

                else if (!strcmp(cmdStr, "QCLEAR")) {
                    stop_count = false;
                    digitalWrite(GatePin, LOW); // Zapnutí hradla
                    noInterrupts();     //  Bezpečné vynulování registrů za běhu
                    integral = 0;
                    interrupts();
                    Serial.println("TOn");
                }

                else if (!strcmp(cmdStr, "QSTOP")) {
                    stop_count = true;
                    digitalWrite(GatePin, HIGH);        // Vypnutí hradla
                }

                else if (!strcmp(cmdStr, "RESET")) {
                    resetFunc();
                }

                else if (!strcmp(cmdStr, "ID?")) {
                    Serial.println(id1);
                }

                else if (!strcmp(cmdStr, "FW?")) {
                    Serial.println(firmware);
                }

                else if (!strcmp(cmdStr, "SET01")) {
                    ttime = 1;
                    Timer1.setPeriod(1000000UL * ttime);        // Bezpečnější změna periody za běhu
                }

                else if (!strcmp(cmdStr, "SET08")) {
                    ttime = 8;
                    Timer1.setPeriod(1000000UL * ttime);        // Bezpečnější změna periody za běhu
                }
            }
        }
    }
}

// Funkce časovače (volá se hardwarově přesně každých X sekund)
void
casovac(void)
{

    // Pracujeme přímo s proměnnou 'pocet'
    proud = long (pocet / ttime);

    // FORMÁT PROUDU "I"
    sprintf(buffer1, "I= %lu.%03lu uA", proud / 1000, proud % 1000);

    // FORMÁT NÁBOJE "Q"
    long charge = long (integral / 1000UL);
    sprintf(buffer2, "Q= %8lu", charge);

    // Kontrola limitu integrálu náboje
    if (charge >= (long) chargemax && !stop_count) {
        stop_count = true;
        digitalWrite(GatePin, HIGH);    // Okamžité hardwarové vypnutí hradla
        triggerTOff = true;     // Signalizace pro loop(), aby vypsal "TOff"
    }

    // Vynulování čítače až po provedení všech výpočtů na konci periody
    pocet = 0;
    dataReady = true;           // Signalizace pro loop(), že buffery jsou naplněny
}

// Funkce externího přerušení (čítání pulsů z hardwarového pinu 2)
void
pocitadlo()
{
    pocet++;
    if (!stop_count) {
        integral++;
    }
}

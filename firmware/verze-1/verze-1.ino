#include <TimerOne.h>
int ttime;
const byte interruptPin = 2;
const byte GatePin = 4;
long pocet, pocet2;
long integral;
long integralmax;
boolean stav;
float proud;
char buffer1[16];
char buffer2[16];
String number = "01";
String id1 = "ID" + number + "nA";
String inString = "";           // string to hold input
boolean stop_count;
void (*resetFunc)(void) = 0;

void
setup(void)
{
    stav = false;
    integral = 0;
    ttime = 1;
    integralmax = 100;
    stop_count = true;
    Serial.begin(9600);
    while (stav == false) {
        while (Serial.available() > 0) {

            int inChar = Serial.read();
            inString += char (inChar);
            int delka = inString.length();
            String funkce = inString.substring(delka - 5, delka);
            String hodnota = inString.substring(0, delka - 5);
            if (funkce == "STMAX") {
                //Serial.println(inString);
                integralmax = hodnota.toInt();
                //Serial.println(hodnota);
                inString = "";
            };

            if (funkce == "RESET") {
                //Serial.println(inString); 
                inString = "";
            }
            if (funkce == "ID???") {
                //Serial.println(inString);
                Serial.println(id1);
                inString = "";
                delay(1000);
            }
            if (funkce == "SETUP") {
                //Serial.println(inString);
                inString = "";
                delay(1000);
            }
            if (funkce == "START") {
                //Serial.println(inString);
                stav = true;
                inString = "";
                delay(1000);
            }
            if (funkce == "SET01") {
                //Serial.println(inString);
                ttime = 1;
                inString = "";
                delay(1000);
            }
            if (funkce == "SET10") {
                //Serial.println(inString);
                ttime = 10;
                inString = "";
                delay(1000);
            }
        }
    }
    pinMode(GatePin, OUTPUT);
    digitalWrite(GatePin, HIGH);
    pinMode(interruptPin, INPUT_PULLUP);
    attachInterrupt(digitalPinToInterrupt(interruptPin), pocitadlo,
                    FALLING);
    Timer1.initialize(2000000 * ttime); //cas 2s
    Timer1.attachInterrupt(casovac);
    pocet = 0;
}

void
loop(void)
{
    long p;

    noInterrupts();
    p = pocet2;
    while (Serial.available() > 0) {
        int inChar = Serial.read();
        inString += char (inChar);
        int delka = inString.length();
        String funkce = inString.substring(delka - 5, delka);
        String hodnota = inString.substring(0, delka - 5);

        if (funkce == "STMAX") {
            //Serial.println(inString);
            integralmax = hodnota.toInt();
            //Serial.println(hodnota);
            inString = "";
        };

        if (funkce == "RESET") {
            inString = "";
            resetFunc();
        }
        if (funkce == "POSLI") {
            inString = "";
            sprintf(buffer2, "I1= %ld.%02ld0 nA", pocet / 100,
                    pocet % 100);
            Serial.println(buffer2);
        }
        if (funkce == "CLEAR") {
            inString = "";
            stop_count = false;
            digitalWrite(GatePin, LOW);
            integral = 0;
        }
        if (funkce == "STOP_") {
            inString = "";
            stop_count = true;
            digitalWrite(GatePin, HIGH);
            integral = 0;
        }
    }
    interrupts();
}

void
casovac(void)
{
    noInterrupts();
    pocet = pocet / ttime;
    sprintf(buffer2, "I1= %ld.%02ld0 nA", pocet / 100, pocet % 100);
    Serial.println(buffer2);
    int C1000 = int (integral / 1000);
    sprintf(buffer2, "C1= %8d", C1000);
    Serial.println(buffer2);
    if (C1000 >= integralmax) {
        stop_count = true;
        digitalWrite(GatePin, HIGH);
        Serial.println("TOff");
    }
    pocet2 = pocet;
    pocet = 0;
    interrupts();
}

void
pocitadlo()
{
    pocet++;
    if (stop_count == false) {
        integral++;
    }
}

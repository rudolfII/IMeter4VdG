#include <TimerOne.h>
#define BAUD_SPEED  9600
#define COMM_LEN  15  // max. length of a serial command
int ttime;
const byte interruptPin = 2;
const byte GatePin = 4;
unsigned long pocet;
unsigned long integral;
unsigned long chargemax;
boolean stop_count;
char buffer1[16];
String firmware="Firmware 1.1 - 20.01.2023";;
String id1="ID002A";

void(* resetFunc) (void) = 0;

void setup(void)
{ integral=0; 
  pocet=0;
  ttime=1;
  chargemax=100;
  stop_count=true;
  Serial.begin(BAUD_SPEED);
  pinMode(GatePin, OUTPUT);
  digitalWrite(GatePin, HIGH);
  pinMode(interruptPin, INPUT_PULLUP);
}

void loop(void)
{ 
int c;
  char comm[COMM_LEN + 1];
  int commPos = 0;
  char *valStr;
  long val;

  for(;;) {
    noInterrupts();
    c = Serial.read();
    if (c != -1) {
      if (c != '\r') {
        if (commPos < COMM_LEN)
          comm[commPos++] = c;
      } else {
        comm[commPos] = '\0';
        commPos = 0;
  
        strtok(comm, " ");
        valStr = strtok(NULL, " ");
        val = strtol(valStr, NULL, 10);
        
        if (!strcmp(comm, "START")){
            attachInterrupt(digitalPinToInterrupt(interruptPin), pocitadlo,FALLING);
            Timer1.initialize(1000000*ttime);
            Timer1.attachInterrupt(casovac);
        }
        else if (!strcmp(comm, "STOP")){
            Timer1.stop();
            detachInterrupt(0);
        }
        else if (!strcmp(comm, "QMAX")){
          chargemax=val;        
        }
        else if (!strcmp(comm, "QCLEAR")) {
          stop_count=false;
          digitalWrite(GatePin, LOW);
          integral=0;
          Serial.println("TOn");  
        }
        else if (!strcmp(comm, "QSTOP")) {
          stop_count=true;
          digitalWrite(GatePin, HIGH);
        }
       else if (!strcmp(comm, "RESET")) {
          resetFunc();
       }
       else if (!strcmp(comm, "ID?")) {
          Serial.println(id1);
       }
       else if (!strcmp(comm, "FW?")) {
          Serial.println(firmware);
       }
       else if (!strcmp(comm, "SET01")) {
          ttime=1;
           Timer1.initialize(1000000*ttime);
       }
       else if (!strcmp(comm, "SET08")) {
          ttime=8; //maximum pro timer1 je cca 8s
           Timer1.initialize(1000000*ttime);
       }
      }
    }
    interrupts();
  }
}

void casovac(void)
{
  noInterrupts();
  pocet=long(pocet/ttime);
  sprintf(buffer1,"I2= %lu.%03lu uA", pocet/1000, pocet%1000); //1kHz corresponds to 1uA, so 1000 counts per 1s corresponds to 1uA 
  Serial.println(buffer1);
  long charge=long(integral/1000UL);  //total charge number of counts / 1000 corresponds to 1uC
  sprintf(buffer1, "Q2= %8lu",charge);
  Serial.println(buffer1);
  if (charge>=chargemax) {
    stop_count=true;
    digitalWrite(GatePin, HIGH);
    Serial.println("TOff");
  }
  pocet=0;
  interrupts();
}

void pocitadlo() {
  pocet++;
  if (stop_count==false) {
        integral++;
  }
}

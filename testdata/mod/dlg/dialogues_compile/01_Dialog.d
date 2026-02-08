BEGIN ~DIALOG_01~

IF ~~ THEN BEGIN Start
  SAY @100
  ++ @110 GOTO Who
  ++ @120 EXIT
END

IF ~~ THEN BEGIN Who
  SAY @101
  ++ ~Back to start.~ GOTO Start
END

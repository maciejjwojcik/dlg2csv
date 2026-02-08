BEGIN ~DIALOG_02~

IF ~~ THEN BEGIN Intro
  SAY @200
  ++ @210 GOTO Details
  ++ @220 EXIT
END

IF ~~ THEN BEGIN Details
  SAY ~This line is a literal SAY without a @ref.~
  ++ @211 EXIT
END

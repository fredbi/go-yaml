* make sure we sanitize bytes for valid UTF-8 input (like our json lexer)
  See spec: https://yaml.org/spec/1.2.2/#51-character-set
  and states that:
  > On input, a YAML processor must support the UTF-8 and UTF-16 character encodings. For JSON compatibility, 
  > the UTF-32 encodings must also be supported.

  Let's start with plain UTF-8

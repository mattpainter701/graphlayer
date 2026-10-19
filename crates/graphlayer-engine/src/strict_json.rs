//! Reject duplicate nested keys before an untyped operation input is inspected.
use serde::de::{self, MapAccess, SeqAccess, Visitor};
use serde::{Deserialize, Deserializer};
use serde_json::{Map, Number, Value};
use std::fmt;

pub struct StrictValue(pub Value);
impl<'de> Deserialize<'de> for StrictValue {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        struct StrictVisitor;
        impl<'de> Visitor<'de> for StrictVisitor {
            type Value = Value;
            fn expecting(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
                f.write_str("unambiguous JSON")
            }
            fn visit_bool<E: de::Error>(self, value: bool) -> Result<Value, E> {
                Ok(Value::Bool(value))
            }
            fn visit_i64<E: de::Error>(self, value: i64) -> Result<Value, E> {
                Ok(Value::Number(value.into()))
            }
            fn visit_u64<E: de::Error>(self, value: u64) -> Result<Value, E> {
                Ok(Value::Number(value.into()))
            }
            fn visit_f64<E: de::Error>(self, value: f64) -> Result<Value, E> {
                Number::from_f64(value)
                    .map(Value::Number)
                    .ok_or_else(|| E::custom("invalid number"))
            }
            fn visit_str<E: de::Error>(self, value: &str) -> Result<Value, E> {
                Ok(Value::String(value.into()))
            }
            fn visit_string<E: de::Error>(self, value: String) -> Result<Value, E> {
                Ok(Value::String(value))
            }
            fn visit_unit<E: de::Error>(self) -> Result<Value, E> {
                Ok(Value::Null)
            }
            fn visit_none<E: de::Error>(self) -> Result<Value, E> {
                Ok(Value::Null)
            }
            fn visit_seq<A: SeqAccess<'de>>(self, mut sequence: A) -> Result<Value, A::Error> {
                let mut values = Vec::new();
                while let Some(StrictValue(value)) = sequence.next_element()? {
                    values.push(value);
                }
                Ok(Value::Array(values))
            }
            fn visit_map<A: MapAccess<'de>>(self, mut object: A) -> Result<Value, A::Error> {
                let mut fields = Map::new();
                while let Some(key) = object.next_key::<String>()? {
                    if fields.contains_key(&key) {
                        return Err(de::Error::custom("duplicate JSON field"));
                    }
                    let StrictValue(value) = object.next_value()?;
                    fields.insert(key, value);
                }
                Ok(Value::Object(fields))
            }
        }
        deserializer.deserialize_any(StrictVisitor).map(StrictValue)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn duplicate_nested_fields_are_rejected() {
        assert!(
            serde_json::from_str::<StrictValue>(r#"{"config":{"secret":1,"secret":2}}"#).is_err()
        );
        assert!(serde_json::from_str::<StrictValue>(r#"[{"x":1,"x":2}]"#).is_err());
        assert!(serde_json::from_str::<StrictValue>(r#"{"x":1} {}"#).is_err());
        assert!(serde_json::from_str::<StrictValue>(r#"{"x":[true,null,2,"text"]}"#).is_ok());
    }
}

// Extend BigInt to support JSON serialization
// BigInt values will be serialized as strings to preserve precision
declare global {
  interface BigInt {
    toJSON(): string;
  }
}

BigInt.prototype.toJSON = function () {
  return this.toString();
};

export {}; // Ensure this is treated as a module

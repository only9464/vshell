// pako 没有自带类型声明；这里给出本项目用到的两处签名。
declare module 'pako' {
  export function inflate(data: Uint8Array | ArrayBuffer): Uint8Array
  export function deflate(data: Uint8Array | ArrayBuffer): Uint8Array
  const pako: { inflate: typeof inflate; deflate: typeof deflate }
  export default pako
}

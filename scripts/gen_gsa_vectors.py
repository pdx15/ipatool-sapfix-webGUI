#!/usr/bin/env python3
"""Generates independent GSA/SRP test vectors for pkg/gsa.

This script is intentionally separate from the Go implementation: it uses the
Python standard library (hashlib/hmac) plus the `cryptography` package for the
AES-CBC reference vector. The vectors are pinned inside pkg/gsa/srp_test.go.
"""
import hashlib
import hmac
import json

# RFC 5054 Appendix A, 2048-bit group (Apple GSA uses this exact modulus).
N_HEX = (
    "ac6bdb41324a9a9bf166de5e1389582faf72b6651987ee07fc3192943db56050"
    "a37329cbb4a099ed8193e0757767a13dd52312ab4b03310dcd7f48a9da04fd50"
    "e8083969edb767b0cf6095179a163ab3661a05fbd5faaae82918a9962f0b93b8"
    "55f97993ec975eeaa80d740adbf4ff747359d041d5c33ea71d281e446b14773b"
    "ca97b43a23fb801676bd207a436c6481f1d2b9078717461a5b9d32e688f87748"
    "544523b524b0d57d5ea77a2775d2ecfa032cfbdbf52fb3786160279004e57ae6"
    "af874e7303ce53299ccc041c7bc308d82a5698f3a8d0c38271ae35f8e9dbfbb6"
    "94b5c803d89f7ae435de236d525f54759b65e372fcd68ef20fa7111f9e4aff73"
)
G = 2
LEN = 256


def H(*parts):
    h = hashlib.sha256()
    for p in parts:
        if isinstance(p, str):
            p = p.encode("utf-8")
        h.update(p)
    return h.digest()


def pad(n, length=LEN):
    return n.to_bytes(length, "big")


def apple_x(password, salt, iters, fo):
    h = hashlib.sha256(password.encode("utf-8")).digest()
    p = h.hex().encode() if fo else h
    pk = hashlib.pbkdf2_hmac("sha256", p, salt, iters, 32)
    h2 = H(b":" + pk)
    return H(salt + h2)


def main():
    N = int(N_HEX, 16)
    n_padded = pad(N)
    g_padded = pad(G)

    out = {
        "n": N_HEX,
        "k": H(n_padded, g_padded).hex(),
        "x_s2k": apple_x("password123", bytes.fromhex("beb25379d1a8581eb5a727673a2441ee"), 20000, False).hex(),
        "x_s2k_fo": apple_x("password123", bytes.fromhex("beb25379d1a8581eb5a727673a2441ee"), 20000, True).hex(),
        "x_s2k_2": apple_x("Test@pple!P4ss", bytes.fromhex("6f8a2c4e1b3d5a7f9c0e2d4b6a8c1e3f"), 11011, False).hex(),
    }

    a = int("60975527035cf2ad1989806f0407210bc81edc04e2762a56afd529ddda2d4393", 16)
    salt = bytes.fromhex("beb25379d1a8581eb5a727673a2441ee")
    B = int(
        "410813e3063f3b4532f2d36413749f39c26c5ceeb1346d3995003c74544c30cb"
        "a318f981281607ae68dbdc3bee9f0544ada6b13d8ac33217b670973152cf03ef"
        "03797615e81dd305342c2e3bb035321d1fd717952e702b09682102d0a5aa25dc"
        "ee01784a32b0684f75626ca3bf8aec874f2dc11f8926944b06f9948e8ad76490"
        "25a58cd9dccdb6b210de00e2283e72baaf93a39b0417dfd1888f841f43d7d41c"
        "75b58f654ccb2e8b9c875c42edc34fd3796200312f2abd19b7e2c54b5702cd1a"
        "7f4d79fdf73bc418c96466ba122d45474ab6db553417715617f6c3b4a8764279"
        "f086acc655e396f85812c90f6f932ce0586168c5deccc9f8beb6891ad13f7caf",
        16,
    )
    B_padded = pad(B)
    A = pow(G, a, N)
    A_padded = pad(A)
    u = H(A_padded, B_padded)
    k = int(out["k"], 16)

    email = "user@example.com"
    x = int.from_bytes(apple_x("correct horse battery staple", salt, 20000, False), "big")
    S = pow((B - k * pow(G, x, N)) % N, a + int.from_bytes(u, "big") * x, N)
    S_padded = pad(S)
    K = H(S_padded)

    hN = H(n_padded)
    hG = H(g_padded)
    xor = bytes(a ^ b for a, b in zip(hN, hG))
    M1 = H(xor + H(email.lower()) + salt + A_padded + B_padded + K)

    out["handshake"] = {
        "a": f"{a:064x}",
        "salt": salt.hex(),
        "B": B_padded.hex(),
        "email": email,
        "iters": 20000,
        "A": A_padded.hex(),
        "u": u.hex(),
        "x": x.to_bytes(32, "big").hex(),  # raw 32-byte SRP private value
        "S": S_padded.hex(),
        "K": K.hex(),
        "M1": M1.hex(),
    }

    out["spd_key"] = hmac.new(K, b"extra data key:", hashlib.sha256).digest().hex()
    out["spd_iv"] = hmac.new(K, b"extra data iv:", hashlib.sha256).digest()[:16].hex()

    # AES-256-CBC reference vector (verified by the Go stdlib test).
    from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
    from cryptography.hazmat.primitives import padding as sympadding

    plaintext = b'<?xml version="1.0" encoding="UTF-8"?>\n<plist version="1.0"><dict><key>foo</key><string>bar</string></dict></plist>'
    key = bytes.fromhex(out["spd_key"])
    iv = bytes.fromhex(out["spd_iv"])
    padder = sympadding.PKCS7(128).padder()
    padded = padder.update(plaintext) + padder.finalize()
    enc = Cipher(algorithms.AES(key), modes.CBC(iv)).encryptor()
    out["cbc"] = {
        "key": key.hex(),
        "iv": iv.hex(),
        "plaintext": plaintext.decode(),
        "ciphertext": (enc.update(padded) + enc.finalize()).hex(),
    }

    print(json.dumps(out, indent=2))


if __name__ == "__main__":
    main()

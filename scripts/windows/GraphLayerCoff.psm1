Set-StrictMode -Version Latest

$script:GraphLayerRustVersion = '1.99.1'
$script:GraphLayerLlvmVersion = '23.1.1'
$script:GraphLayerRustHost = 'x86_64-pc-windows-msvc'
$script:GraphLayerAbsoluteSection = -1
$script:GraphLayerStaticStorageClass = 3

function Resolve-GraphLayerObjcopyAtSysroot([string] $Sysroot, [string] $HostTriple) {
    if ([string]::IsNullOrWhiteSpace($Sysroot) -or $HostTriple -ne $script:GraphLayerRustHost) {
        throw 'GraphLayer requires the pinned x86_64-pc-windows-msvc Rust toolchain.'
    }
    $candidate = Join-Path $Sysroot "lib/rustlib/$HostTriple/bin/llvm-objcopy.exe"
    if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) {
        throw 'The pinned Rust llvm-tools-preview component is required (llvm-objcopy.exe was not found in the Rust sysroot).'
    }
    return [IO.Path]::GetFullPath($candidate)
}

function Resolve-GraphLayerPinnedObjcopy {
    $rustup = Get-Command rustup.exe -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $rustup -or -not $rustup.Source) {
        throw 'Rustup is required to resolve llvm-objcopy from the pinned Rust toolchain.'
    }
    $toolchain = "$script:GraphLayerRustVersion-$script:GraphLayerRustHost"
    $versionLines = @(& $rustup.Source run $toolchain rustc --version --verbose 2>$null)
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect the selected Rust toolchain.' }
    $releaseLine = $versionLines | Where-Object { $_ -match '^release: ' } | Select-Object -First 1
    $hostLine = $versionLines | Where-Object { $_ -match '^host: ' } | Select-Object -First 1
    if (-not $releaseLine -or $releaseLine -ne "release: $script:GraphLayerRustVersion" -or
        -not $hostLine -or $hostLine -ne "host: $script:GraphLayerRustHost") {
        throw 'GraphLayer requires Rust 1.99.1 for x86_64-pc-windows-msvc.'
    }
    $sysrootLines = @(& $rustup.Source run $toolchain rustc --print sysroot 2>$null)
    if ($LASTEXITCODE -ne 0 -or $sysrootLines.Count -ne 1) {
        throw 'Could not resolve the pinned Rust sysroot.'
    }
    $objcopy = Resolve-GraphLayerObjcopyAtSysroot -Sysroot $sysrootLines[0].Trim() -HostTriple $script:GraphLayerRustHost
    $versionOutput = @(& $objcopy --version 2>$null) -join "`n"
    $expectedLlvmVersion = "LLVM version $script:GraphLayerLlvmVersion-rust-$script:GraphLayerRustVersion-stable"
    if ($LASTEXITCODE -ne 0 -or $versionOutput -notmatch ([regex]::Escape($expectedLlvmVersion) + '(?:\s|$)')) {
        throw 'The Rust sysroot llvm-objcopy version does not match the pinned LLVM toolchain.'
    }
    return $objcopy
}

function Read-CoffUInt16([byte[]] $Bytes, [int] $Offset) {
    if ($Offset -lt 0 -or $Offset + 2 -gt $Bytes.Length) { throw 'COFF metadata is truncated.' }
    return [BitConverter]::ToUInt16($Bytes, $Offset)
}

function Read-CoffInt16([byte[]] $Bytes, [int] $Offset) {
    if ($Offset -lt 0 -or $Offset + 2 -gt $Bytes.Length) { throw 'COFF metadata is truncated.' }
    return [BitConverter]::ToInt16($Bytes, $Offset)
}

function Read-CoffUInt32([byte[]] $Bytes, [int] $Offset) {
    if ($Offset -lt 0 -or $Offset + 4 -gt $Bytes.Length) { throw 'COFF metadata is truncated.' }
    return [BitConverter]::ToUInt32($Bytes, $Offset)
}

function Get-CoffString([byte[]] $Bytes, [int] $Start, [int] $Limit) {
    if ($Start -lt 0 -or $Start -ge $Limit -or $Limit -gt $Bytes.Length) { throw 'COFF string offset is invalid.' }
    $end = $Start
    while ($end -lt $Limit -and $Bytes[$end] -ne 0) { $end++ }
    if ($end -eq $Limit) { throw 'COFF string is not terminated.' }
    $value = [byte[]]::new($end - $Start)
    if ($value.Length) { [Array]::Copy($Bytes, $Start, $value, 0, $value.Length) }
    return @{ Text = [Text.Encoding]::ASCII.GetString($value); Hex = [Convert]::ToHexString($value) }
}

function Get-CoffName([byte[]] $Bytes, [int] $Offset, [int] $Width, [int] $StringStart, [int] $StringEnd, [switch] $SectionName) {
    $field = [byte[]]::new($Width)
    [Array]::Copy($Bytes, $Offset, $field, 0, $Width)
    $inline = [Text.Encoding]::ASCII.GetString($field).TrimEnd([char]0)
    if ($SectionName -and $inline -match '^/([0-9]+)$') {
        $token = $Matches[1]
        $string = Get-CoffString $Bytes ($StringStart + [int]$token) $StringEnd
        return $string
    }
    if (-not $SectionName -and (Read-CoffUInt32 $field 0) -eq 0) {
        $nameOffset = Read-CoffUInt32 $field 4
        if ($StringStart -eq 0 -or $nameOffset -lt 4) { throw 'COFF symbol-name offset is invalid.' }
        return (Get-CoffString $Bytes ($StringStart + [int]$nameOffset) $StringEnd)
    }
    $end = 0
    while ($end -lt $Width -and $field[$end] -ne 0) { $end++ }
    $nameBytes = [byte[]]::new($end)
    if ($end) { [Array]::Copy($field, 0, $nameBytes, 0, $end) }
    return @{ Text = [Text.Encoding]::ASCII.GetString($nameBytes); Hex = [Convert]::ToHexString($nameBytes) }
}

function Get-CoffHash([byte[]] $Bytes) {
    return [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($Bytes)).ToLowerInvariant()
}

function Get-CoffSymbolIdentity([object] $Symbol) {
    return '{0}|{1}|{2}|{3}|{4}|{5}' -f $Symbol.NameHex, $Symbol.Value, $Symbol.Section, $Symbol.Type, $Symbol.StorageClass, $Symbol.AuxHex
}

function Test-CoffSymbolEquivalent([object] $BeforeSymbol, [object] $AfterSymbol, [object[]] $BeforeSections) {
    if ((Get-CoffSymbolIdentity $BeforeSymbol) -ceq (Get-CoffSymbolIdentity $AfterSymbol)) { return $true }
    foreach ($field in @('NameHex', 'Value', 'Section', 'Type', 'StorageClass', 'AuxCount')) {
        if ($BeforeSymbol[$field] -cne $AfterSymbol[$field]) { return $false }
    }
    if ($BeforeSymbol.AuxCount -ne 1 -or $BeforeSymbol.Section -le 0 -or $BeforeSymbol.Section -gt $BeforeSections.Count -or
        $BeforeSymbol.StorageClass -ne $script:GraphLayerStaticStorageClass -or $BeforeSymbol.Value -ne 0 -or $BeforeSymbol.Type -ne 0) { return $false }
    $section = $BeforeSections[$BeforeSymbol.Section - 1]
    if ($BeforeSymbol.NameHex -cne $section.NameHex) { return $false }
    try {
        $beforeAux = [Convert]::FromHexString($BeforeSymbol.AuxHex)
        $afterAux = [Convert]::FromHexString($AfterSymbol.AuxHex)
        if ($beforeAux.Length -ne 18 -or $afterAux.Length -ne 18) { return $false }
        $auxLength = Read-CoffUInt32 $beforeAux 0
        $auxRelocations = Read-CoffUInt16 $beforeAux 4
        $auxLines = Read-CoffUInt16 $beforeAux 6
        $beforeNumber = Read-CoffUInt16 $beforeAux 12
        $afterNumber = Read-CoffUInt16 $afterAux 12
        if ($auxLength -ne $section.RawSize -or $auxRelocations -ne $section.Relocations.Count -or
            $auxLines -ne $section.LineCount -or $beforeAux[14] -ne 0 -or
            ($beforeNumber -ne 0 -and $beforeNumber -ne $BeforeSymbol.Section)) { return $false }
        if ($beforeNumber -eq $BeforeSymbol.Section -and $afterNumber -ne $beforeNumber) { return $false }
        if ($beforeNumber -eq 0 -and $afterNumber -ne 0 -and $afterNumber -ne $BeforeSymbol.Section) { return $false }
        $beforeAux[12] = 0
        $beforeAux[13] = 0
        $afterAux[12] = 0
        $afterAux[13] = 0
        return [Convert]::ToHexString($beforeAux) -ceq [Convert]::ToHexString($afterAux)
    } catch {
        return $false
    }
}

function Get-CoffSymbolAtIndex([object] $Metadata, [uint32] $Index) {
    $symbol = @($Metadata.Symbols | Where-Object { $_.Index -eq $Index }) | Select-Object -First 1
    if (-not $symbol) { throw 'COFF relocation target is not a primary symbol.' }
    return $symbol
}

function Get-GraphLayerCoffMetadata([string] $Path) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'COFF input must be a regular non-reparse file.' }
    $bytes = [IO.File]::ReadAllBytes($item.FullName)
    if ($bytes.Length -lt 20) { throw 'COFF object is truncated.' }
    $machine = Read-CoffUInt16 $bytes 0
    $sectionCount = Read-CoffUInt16 $bytes 2
    $timestamp = Read-CoffUInt32 $bytes 4
    $symbolPointer = Read-CoffUInt32 $bytes 8
    $symbolSlots = Read-CoffUInt32 $bytes 12
    $optionalSize = Read-CoffUInt16 $bytes 16
    $characteristics = Read-CoffUInt16 $bytes 18
    if ($sectionCount -eq 0 -or $sectionCount -gt 96 -or $optionalSize -ne 0) { throw 'COFF object header is unsupported.' }
    $sectionTableEnd = 20 + ([int]$sectionCount * 40)
    if ($sectionTableEnd -gt $bytes.Length) { throw 'COFF section table is truncated.' }
    if ($symbolSlots -eq 0 -or $symbolPointer -lt $sectionTableEnd) { throw 'COFF symbol table is missing or misplaced.' }
    $symbolEnd64 = [int64]$symbolPointer + ([int64]$symbolSlots * 18)
    if ($symbolEnd64 + 4 -gt $bytes.Length) { throw 'COFF symbol table is truncated.' }
    $stringStart = [int]$symbolEnd64
    $stringSize = Read-CoffUInt32 $bytes $stringStart
    # CVTRES uses a zero length word for an empty string table. Treat it as
    # empty, but any section/symbol long-name reference still fails bounds.
    if ($stringSize -eq 0) {
        $stringEnd = $stringStart + 4
        $stringPayloadLength = 0
        $stringPayloadHash = Get-CoffHash ([byte[]]::new(0))
    } else {
        if ($stringSize -lt 4 -or [int64]$stringStart + $stringSize -gt $bytes.Length) { throw 'COFF string table is malformed.' }
        $stringEnd = $stringStart + [int]$stringSize
        $stringPayloadLength = [int]$stringSize - 4
        $stringPayload = [byte[]]::new($stringPayloadLength)
        if ($stringPayloadLength) { [Array]::Copy($bytes, $stringStart + 4, $stringPayload, 0, $stringPayloadLength) }
        $stringPayloadHash = Get-CoffHash $stringPayload
    }

    $symbols = [Collections.Generic.List[object]]::new()
    $symbolByIndex = @{}
    $index = 0
    while ($index -lt $symbolSlots) {
        $offset = [int]$symbolPointer + ($index * 18)
        $name = Get-CoffName $bytes $offset 8 $stringStart $stringEnd
        $value = Read-CoffUInt32 $bytes ($offset + 8)
        $section = Read-CoffInt16 $bytes ($offset + 12)
        $type = Read-CoffUInt16 $bytes ($offset + 14)
        $storage = $bytes[$offset + 16]
        $auxCount = [int]$bytes[$offset + 17]
        if ($index + $auxCount -ge $symbolSlots) { throw 'COFF auxiliary symbol count is invalid.' }
        $auxBytes = [byte[]]::new($auxCount * 18)
        if ($auxBytes.Length) { [Array]::Copy($bytes, $offset + 18, $auxBytes, 0, $auxBytes.Length) }
        $symbol = [ordered]@{ Index = $index; Name = $name.Text; NameHex = $name.Hex; Value = $value; Section = $section; Type = $type; StorageClass = $storage; AuxCount = $auxCount; AuxHex = [Convert]::ToHexString($auxBytes) }
        $symbolByIndex[$index] = $symbol
        $symbols.Add($symbol)
        for ($auxIndex = 1; $auxIndex -le $auxCount; $auxIndex++) { $symbolByIndex[$index + $auxIndex] = $null }
        $index += 1 + $auxCount
    }
    if ($index -ne $symbolSlots) { throw 'COFF symbol table records are malformed.' }

    $sections = [Collections.Generic.List[object]]::new()
    for ($sectionIndex = 0; $sectionIndex -lt $sectionCount; $sectionIndex++) {
        $offset = 20 + ($sectionIndex * 40)
        $name = Get-CoffName $bytes $offset 8 $stringStart $stringEnd -SectionName
        $virtualSize = Read-CoffUInt32 $bytes ($offset + 8)
        $virtualAddress = Read-CoffUInt32 $bytes ($offset + 12)
        $rawSize = Read-CoffUInt32 $bytes ($offset + 16)
        $rawPointer = Read-CoffUInt32 $bytes ($offset + 20)
        $relocationPointer = Read-CoffUInt32 $bytes ($offset + 24)
        $linePointer = Read-CoffUInt32 $bytes ($offset + 28)
        $relocationCount = Read-CoffUInt16 $bytes ($offset + 32)
        $lineCount = Read-CoffUInt16 $bytes ($offset + 34)
        $flags = Read-CoffUInt32 $bytes ($offset + 36)
        if ($rawSize -gt 0 -and ([int64]$rawPointer + $rawSize -gt $bytes.Length)) { throw 'COFF section data is out of bounds.' }
        if ($relocationCount -eq 0xffff -or ($relocationCount -gt 0 -and ([int64]$relocationPointer + ([int64]$relocationCount * 10) -gt $bytes.Length))) {
            throw 'COFF relocation table is unsupported or out of bounds.'
        }
        if ($lineCount -gt 0 -and ([int64]$linePointer + ([int64]$lineCount * 6) -gt $bytes.Length)) { throw 'COFF line-number table is out of bounds.' }
        $raw = [byte[]]::new([int]$rawSize)
        if ($raw.Length) { [Array]::Copy($bytes, [int]$rawPointer, $raw, 0, $raw.Length) }
        $relocations = [Collections.Generic.List[object]]::new()
        for ($relocationIndex = 0; $relocationIndex -lt $relocationCount; $relocationIndex++) {
            $relocationOffset = [int]$relocationPointer + ($relocationIndex * 10)
            $address = Read-CoffUInt32 $bytes $relocationOffset
            $targetIndex = Read-CoffUInt32 $bytes ($relocationOffset + 4)
            $relocationType = Read-CoffUInt16 $bytes ($relocationOffset + 8)
            if (-not $symbolByIndex.ContainsKey([int]$targetIndex) -or $null -eq $symbolByIndex[[int]$targetIndex]) {
                throw 'COFF relocation references an invalid or auxiliary symbol index.'
            }
            $relocations.Add([ordered]@{ Address = $address; TargetIndex = $targetIndex; Type = $relocationType })
        }
        $sections.Add([ordered]@{ Index = $sectionIndex + 1; Name = $name.Text; NameHex = $name.Hex; VirtualSize = $virtualSize; VirtualAddress = $virtualAddress; RawSize = $rawSize; RawPointer = $rawPointer; RawHash = Get-CoffHash $raw; RelocationPointer = $relocationPointer; Relocations = @($relocations); LinePointer = $linePointer; LineCount = $lineCount; Flags = $flags })
    }
    return [ordered]@{ Path = $item.FullName; Machine = $machine; Timestamp = $timestamp; SectionCount = $sectionCount; SymbolPointer = $symbolPointer; SymbolSlots = $symbolSlots; StringPayloadLength = $stringPayloadLength; StringPayloadHash = $stringPayloadHash; OptionalHeaderSize = $optionalSize; Characteristics = $characteristics; Symbols = @($symbols); Sections = @($sections) }
}

function Assert-GraphLayerCoffInput([object] $Metadata) {
    if ($Metadata.Machine -ne 0x8664) { throw 'Go resource COFF object must target AMD64.' }
    $markers = @($Metadata.Symbols | Where-Object { $_.Name -ceq '@comp.id' })
    if ($markers.Count -ne 1) { throw 'Go resource COFF object must contain exactly one @comp.id marker.' }
    $marker = $markers[0]
    if ($marker.Section -ne $script:GraphLayerAbsoluteSection -or $marker.StorageClass -ne $script:GraphLayerStaticStorageClass -or
        $marker.Type -ne 0 -or $marker.AuxCount -ne 0) {
        throw 'Go resource @comp.id marker has an unexpected COFF shape.'
    }
    foreach ($section in $Metadata.Sections) {
        # CVTRES version-resource objects have no COFF line-number records.
        # Reject broader inputs so no unchecked line-table data can be promoted.
        if ($section.LineCount -ne 0 -or $section.LinePointer -ne 0) {
            throw 'Go version-resource COFF objects must not contain line-number tables.'
        }
        if (@($section.Relocations | Where-Object { $_.TargetIndex -eq $marker.Index }).Count -gt 0) {
            throw 'Go resource @comp.id marker is referenced by a COFF relocation.'
        }
    }
    return $marker
}

function Assert-GraphLayerCoffNormalized([object] $Before, [object] $After) {
    $marker = Assert-GraphLayerCoffInput $Before
    if ($After.Machine -ne 0x8664 -or $After.SectionCount -ne $Before.SectionCount -or
        $After.Timestamp -ne $Before.Timestamp -or $After.SymbolPointer -ne $Before.SymbolPointer -or
        $After.StringPayloadLength -ne $Before.StringPayloadLength -or $After.StringPayloadHash -cne $Before.StringPayloadHash -or
        $After.Characteristics -ne $Before.Characteristics -or $After.OptionalHeaderSize -ne 0) {
        throw 'Normalized Go resource COFF header changed unexpectedly.'
    }
    if ($After.SymbolSlots -ne ($Before.SymbolSlots - 1)) { throw 'Normalization changed an unexpected number of COFF symbol records.' }
    if (@($After.Symbols | Where-Object { $_.Name -ceq '@comp.id' }).Count -ne 0) { throw 'Normalized Go resource still contains @comp.id.' }
    $expectedSymbols = @($Before.Symbols | Where-Object { $_.Index -ne $marker.Index })
    if ($expectedSymbols.Count -ne $After.Symbols.Count) { throw 'Normalization changed symbols other than @comp.id.' }
    for ($i = 0; $i -lt $expectedSymbols.Count; $i++) {
        if (-not (Test-CoffSymbolEquivalent $expectedSymbols[$i] $After.Symbols[$i] $Before.Sections)) {
            throw 'Normalization changed symbols other than @comp.id.'
        }
    }
    if ($Before.Sections.Count -ne $After.Sections.Count) { throw 'Normalization changed the COFF section count.' }
    for ($i = 0; $i -lt $Before.Sections.Count; $i++) {
        $left = $Before.Sections[$i]
        $right = $After.Sections[$i]
        foreach ($field in @('NameHex','VirtualSize','VirtualAddress','RawSize','RawPointer','RawHash','RelocationPointer','LinePointer','LineCount','Flags')) {
            if ($left[$field] -cne $right[$field]) { throw "Normalization changed COFF section metadata ($($left.Name))." }
        }
        if ($left.Relocations.Count -ne $right.Relocations.Count) { throw "Normalization changed relocations in $($left.Name)." }
        for ($j = 0; $j -lt $left.Relocations.Count; $j++) {
            foreach ($field in @('Address','Type')) {
                if ($left.Relocations[$j][$field] -cne $right.Relocations[$j][$field]) { throw "Normalization changed a relocation in $($left.Name)." }
            }
            $leftTarget = Get-CoffSymbolAtIndex $Before ([uint32]$left.Relocations[$j].TargetIndex)
            $rightTarget = Get-CoffSymbolAtIndex $After ([uint32]$right.Relocations[$j].TargetIndex)
            if (-not (Test-CoffSymbolEquivalent $leftTarget $rightTarget $Before.Sections)) {
                throw "Normalization changed a relocation target in $($left.Name)."
            }
        }
    }
}

function Normalize-GraphLayerGoResource([string] $InputPath, [string] $OutputPath, [string] $ObjcopyPath) {
    if (Test-Path -LiteralPath $OutputPath) { throw 'Normalized Go resource output must be new.' }
    $before = Get-GraphLayerCoffMetadata $InputPath
    $null = Assert-GraphLayerCoffInput $before
    $resolvedObjcopy = [IO.Path]::GetFullPath($ObjcopyPath)
    if (-not (Test-Path -LiteralPath $resolvedObjcopy -PathType Leaf)) { throw 'The pinned Rust llvm-objcopy executable is unavailable.' }
    & $resolvedObjcopy '--strip-symbol=@comp.id' $before.Path ([IO.Path]::GetFullPath($OutputPath)) *> $null
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0 -or -not (Test-Path -LiteralPath $OutputPath -PathType Leaf)) {
        throw "llvm-objcopy could not normalize the Go resource COFF object (exit code $exitCode)."
    }
    try {
        $after = Get-GraphLayerCoffMetadata $OutputPath
        Assert-GraphLayerCoffNormalized $before $after
    } catch {
        Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
        throw
    }
    return $after
}

Export-ModuleMember -Function Resolve-GraphLayerObjcopyAtSysroot, Resolve-GraphLayerPinnedObjcopy, Get-GraphLayerCoffMetadata, Assert-GraphLayerCoffInput, Assert-GraphLayerCoffNormalized, Normalize-GraphLayerGoResource

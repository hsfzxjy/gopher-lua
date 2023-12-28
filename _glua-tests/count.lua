local function countOccurrences(str)
    local charCount = {}
    for i = 1, #str do
        local char = string.sub(str, i, i)
        charCount[char] = (charCount[char] or 0) + 1
    end
    return charCount
end

local text = [=[
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Aliquam rhoncus sed nulla vitae commodo. Maecenas vitae felis nisi. Curabitur ac augue fringilla, ornare eros ut, ultricies urna. Quisque dictum facilisis leo, ut porta tortor. Praesent aliquam enim neque, id sollicitudin eros eleifend molestie. Aliquam ac vestibulum urna. Mauris tincidunt convallis elit ac molestie.

Nam ac volutpat nisl, ut porttitor diam. Nam quam erat, vestibulum ut interdum non, malesuada et massa. Sed sed elit aliquam, sollicitudin nunc at, viverra odio. Suspendisse ac risus at arcu dapibus commodo. Suspendisse eget hendrerit diam, ac accumsan enim. Vivamus consectetur ut neque facilisis sagittis. Morbi sed leo urna. Integer vitae felis dolor. Nullam lobortis, tortor eu placerat cursus, dui diam semper eros, ac finibus lectus elit eget lorem. Integer aliquam nibh ut ligula tincidunt venenatis. Aliquam libero est, aliquam eu sollicitudin non, egestas eu augue. Maecenas ac malesuada nulla, at pulvinar metus. Vestibulum interdum placerat lacinia. Cras at tellus nec ex rutrum convallis. Sed ac ultricies lacus. In aliquet ullamcorper nibh.

Duis vel placerat enim, accumsan sagittis lacus. Mauris condimentum sem eu mauris bibendum, sed sodales metus lobortis. Donec quis augue lobortis, tristique enim vitae, pellentesque mauris. Quisque ultrices, risus quis pellentesque accumsan, odio metus sagittis massa, a blandit lacus arcu ut odio. Etiam eu ante at ex viverra scelerisque sit amet sit amet lorem. Nam turpis dolor, tristique et auctor nec, posuere et enim. Maecenas egestas sollicitudin felis, at tristique sem sodales tincidunt. Proin bibendum molestie nunc et semper. Donec a velit finibus, porta dolor in, aliquet risus. Quisque interdum nibh ut elit tempor porttitor. Sed dolor metus, sollicitudin quis molestie sed, mattis at nulla. Aenean dolor urna, tincidunt vel purus vel, cursus semper lorem. Ut hendrerit dictum nisi ac dignissim. In velit nulla, gravida vitae euismod ut, iaculis at eros.

Fusce id gravida arcu. Integer ut bibendum purus, ut commodo mauris. In sapien lorem, egestas et sem at, rutrum placerat mauris. Nunc sem eros, vehicula nec ipsum quis, vehicula ultrices orci. Donec condimentum luctus tempor. Pellentesque iaculis purus at dictum pulvinar. Curabitur ut tristique metus, eu faucibus lectus. In et odio volutpat, ultrices magna eget, condimentum massa. Quisque porttitor enim dolor, et commodo nibh tincidunt in. Ut dictum ligula id aliquet accumsan. Mauris est ligula, luctus vitae nisl quis, feugiat facilisis turpis. Aenean porta metus id aliquet pulvinar. Proin dignissim mollis rutrum. Mauris in aliquam tellus, nec tempor arcu.

Phasellus malesuada dolor et tortor feugiat, eu pharetra nisi eleifend. In tempus orci in odio maximus molestie. Maecenas elit diam, tincidunt id malesuada non, fringilla sit amet leo. Mauris tellus augue, ultricies sed ligula quis, tincidunt malesuada dolor. Suspendisse nec ante vitae nunc rutrum consectetur. Nullam aliquet accumsan mi, sed condimentum eros consectetur ullamcorper. Aenean suscipit libero non elementum condimentum. Aliquam at neque id lectus accumsan dictum. Mauris quam arcu, efficitur quis luctus vel, finibus non purus.
]=]

for i = 1, 5000 do countOccurrences(text) end
